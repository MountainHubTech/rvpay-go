package ghlsync

import (
	"context"
	"time"

	clientsgrpc "github.com/MountainHubTech/rvpay-go/grpc/go/clientsgrpc"
	"github.com/MountainHubTech/rvpay-go/transactions/db/repo"
	"github.com/MountainHubTech/rvpay-go/transactions/db/sqlc"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// MaxGhlSyncAttempts is the total number of GHL update attempts per deposit
// (one initial attempt plus one retry).
const MaxGhlSyncAttempts int32 = 2

// DefaultPollInterval is the pause between outbox polls when empty.
//
// A single statement inside PostgreSQL is one atomic database transaction, so
// the deposit status update and the synchronization intent commit together;
// the external GHL call is always made after the commit, never inside it.
const DefaultPollInterval = 10 * time.Second

// clientNamePrefix is the RVPay client-name convention binding a deposit to
// its HighLevel location ("highlevel-<locationId>").
const clientNamePrefix = "highlevel-"

// Worker is the durable GHL order-status synchronization worker. It polls the
// deposit outbox (ghl_sync_status='pending' on terminal PawaPay deposits),
// claims rows atomically, and sends the GHL request through the Clients
// PaymentSyncService after the deposit transaction has committed.
//
// Retry behavior (simple two-try):
//   - attempt fails and fewer than MaxGhlSyncAttempts have been made -> row
//     is re-queued to 'pending' for exactly one more try;
//   - attempt fails and MaxGhlSyncAttempts have been made -> the row is
//     recorded 'failed' with the error and timestamp; the PawaPay status
//     never changes and the PawaPay callback already returned success.
//
// Keep the implementation simple: no job framework, no broker, no Redis.
// Shutdown is cooperative via ctx cancellation.
type Worker struct {
	depositRepo repo.DepositRepo
	// syncClient performs the Clients-side GHL update. It is set by connect
	// once; tests drive RunOnce after injecting a fake through connectOnce.
	syncClient   clientsgrpc.PaymentSyncServiceClient
	clientsAddr  string
	logger       zerolog.Logger
	pollInterval time.Duration
}

// NewWorker creates a synchronization worker that connects to the Clients
// service at clientsAddr (CLIENTS_GRPC_ADDR configuration; never hard-coded).
func NewWorker(depositRepo repo.DepositRepo, clientsAddr string, logger zerolog.Logger, pollInterval time.Duration) *Worker {
	if pollInterval <= 0 {
		pollInterval = DefaultPollInterval
	}
	return &Worker{
		depositRepo:  depositRepo,
		clientsAddr:  clientsAddr,
		logger:       logger,
		pollInterval: pollInterval,
	}
}

// connect lazily dials the Clients service and builds the sync client.
func (w *Worker) connect(ctx context.Context) error {
	if w.syncClient != nil {
		return nil
	}
	if w.clientsAddr == "" {
		return status.Error(codes.FailedPrecondition, "clients grpc address is not configured")
	}
	conn, err := grpc.NewClient(w.clientsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return status.Errorf(codes.Internal, "connect to clients service: %v", err)
	}
	w.syncClient = clientsgrpc.NewPaymentSyncServiceClient(conn)
	return nil
}

// Run polls and processes the outbox until ctx is cancelled. It blocks; run
// it in a goroutine alongside the servers.
func (w *Worker) Run(ctx context.Context) {
	w.logger.Info().Msg("GHL order-status synchronization worker started")
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	for {
		if err := w.RunOnce(ctx); err != nil {
			w.logger.Error().Err(err).Msg("GHL synchronization poll failed")
		}
		select {
		case <-ctx.Done():
			w.logger.Info().Msg("GHL order-status synchronization worker stopped")
			return
		case <-ticker.C:
		}
	}
}

// RunOnce claims all pending rows and processes them.
func (w *Worker) RunOnce(ctx context.Context) error {
	if err := w.connect(ctx); err != nil {
		return err
	}
	claimed, err := w.depositRepo.ClaimPendingGhlSync(ctx)
	if err != nil {
		return err
	}
	for _, deposit := range claimed {
		w.processOne(ctx, deposit)
	}
	return nil
}

// processOne performs one claimed GHL update: it sends the request after the
// deposit transaction has committed, then records success, a single retry, or
// the final failure. The PawaPay-authoritative deposit status is never
// changed here.
func (w *Worker) processOne(ctx context.Context, deposit sqlc.Deposit) {
	depositID := deposit.ID.String()
	var orderID string
	if deposit.GhlOrderID != nil {
		orderID = *deposit.GhlOrderID
	}
	locationID := locationIDFromClientName(deposit.ClientName)

	targetStatus := syncTargetStatus(deposit.Status)
	if targetStatus == "" || orderID == "" || locationID == "" {
		// No valid outbound target: mark failed immediately so the worker
		// does not retry malformed rows (a missing identifier is not
		// transient). The deposit status itself is untouched.
		w.logger.Warn().
			Str("deposit_id", depositID).
			Str("order_id", orderID).
			Str("location_id", locationID).
			Str("deposit_status", string(deposit.Status)).
			Msg("GHL synchronization row has no valid outbound target; recording failure")
		if _, err := w.depositRepo.MarkGhlSyncFailure(ctx, deposit.ID, "missing or unsupported GHL synchronization identifiers"); err != nil {
			w.logger.Error().Err(err).Str("deposit_id", depositID).Msg("could not record GHL synchronization failure")
		}
		return
	}

	_, err := w.syncClient.UpdateGhlOrderStatus(ctx, &clientsgrpc.UpdateGhlOrderStatusRequest{
		LocationId: locationID,
		OrderId:    orderID,
		Status:     targetStatus,
	})
	if err == nil {
		if _, recErr := w.depositRepo.MarkGhlSyncSuccess(ctx, deposit.ID); recErr != nil {
			// The GHL update succeeded but the local record failed; retrying
			// the DB record matters more than logging. Re-queue the row so
			// the database state cannot strand an accepted update in
			// 'processing'. The deposit status is never touched.
			w.logger.Error().Err(recErr).Str("deposit_id", depositID).Str("order_id", orderID).Msg("GHL update accepted but success could not be recorded")
			w.requeueAfterRecordFailure(ctx, deposit, "ghl update accepted but success record failed")
			return
		}
		w.logger.Info().
			Str("deposit_id", depositID).
			Str("location_id", locationID).
			Str("order_id", orderID).
			Str("status", targetStatus).
			Msg("GHL order status updated")
		return
	}

	// One failure: retry once. The second failure is final.
	if deposit.GhlSyncAttempts < MaxGhlSyncAttempts {
		w.logger.Warn().Err(err).
			Str("deposit_id", depositID).
			Str("order_id", orderID).
			Int32("attempt", deposit.GhlSyncAttempts).
			Msg("GHL order status update failed; re-queued for one retry")
		// Attempts were already incremented by the claim; re-queue to
		// 'pending' for the single retry.
		if _, recErr := w.depositRepo.MarkGhlSyncRetry(ctx, deposit.ID, sanitizeGhlError(err)); recErr != nil {
			w.logger.Error().Err(recErr).Str("deposit_id", depositID).Msg("could not re-queue GHL synchronization")
		}
		return
	}

	w.logger.Error().Err(err).
		Str("deposit_id", depositID).
		Str("location_id", locationID).
		Str("order_id", orderID).
		Str("status", targetStatus).
		Msg("GHL order status update failed twice; preserving PawaPay deposit status and recording failure")
	if _, recErr := w.depositRepo.MarkGhlSyncFailure(ctx, deposit.ID, sanitizeGhlError(err)); recErr != nil {
		w.logger.Error().Err(recErr).Str("deposit_id", depositID).Msg("could not record GHL synchronization failure")
	}
}

// requeueAfterRecordFailure returns a row whose success record failed to
// 'pending' so the database state cannot strand an accepted GHL update in
// 'processing'. This must never alter the deposit status.
func (w *Worker) requeueAfterRecordFailure(ctx context.Context, deposit sqlc.Deposit, reason string) {
	if _, err := w.depositRepo.MarkGhlSyncRetry(ctx, deposit.ID, reason); err != nil {
		w.logger.Error().Err(err).Str("deposit_id", deposit.ID.String()).Msg("could not re-queue GHL synchronization after record failure")
	}
}

// syncTargetStatus maps the PawaPay-authoritative deposit status onto the GHL
// order status. Only terminal states produce a target; pending, processing,
// unknown, or unsupported states return "" and never trigger an update.
func syncTargetStatus(st sqlc.DepositStatus) string {
	switch st {
	case sqlc.DepositStatusCOMPLETED:
		return "completed"
	case sqlc.DepositStatusFAILED:
		return "failed"
	default:
		return ""
	}
}

// locationIDFromClientName derives the HighLevel locationId from the RVPay
// client-name convention "highlevel-<locationId>". It returns "" for any
// non-conforming name so the row is never synced against a fabricated
// location.
func locationIDFromClientName(clientName string) string {
	if len(clientName) <= len(clientNamePrefix) {
		return ""
	}
	if clientName[:len(clientNamePrefix)] != clientNamePrefix {
		return ""
	}
	return clientName[len(clientNamePrefix):]
}

// sanitizeGhlError strips an outbound GHL error to its message. Provider
// errors are already sanitized server-side (credential-redacted); no tokens
// ever pass through here.
func sanitizeGhlError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
