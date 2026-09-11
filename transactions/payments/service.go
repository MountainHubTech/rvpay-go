package payments

import (
	"context"
	"errors"
	"strings"

	transactionsgrpc "github.com/MountainHubTech/rvpay-go/grpc/go/transactionsgrpc"
	"github.com/MountainHubTech/rvpay-go/transactions/db/repo"
	"github.com/MountainHubTech/rvpay-go/transactions/db/sqlc"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Impl owns the payment-provider backend contract for the GHL Custom Payment
// Provider integration. It implements payment verification (the verify query
// operation) and payment webhook business processing. Payment-domain
// decisions live here, not in Clients; the GHL-facing transport adapters in
// Clients delegate to this service via gRPC.
type Impl struct {
	depositRepo      repo.DepositRepo
	transactionsRepo repo.TransactionsRepo
	logger           zerolog.Logger

	transactionsgrpc.UnimplementedPaymentServiceServer
}

// NewPaymentService creates a new payment-provider service.
func NewPaymentService(
	depositRepo repo.DepositRepo,
	transactionsRepo repo.TransactionsRepo,
	logger zerolog.Logger,
) *Impl {
	return &Impl{
		depositRepo:      depositRepo,
		transactionsRepo: transactionsRepo,
		logger:           logger,
	}
}

// VerifyPayment verifies whether a referenced payment has succeeded. It
// looks up the deposit by its GoHighLevel transaction identifier (or charge
// identifier as a fallback) and interprets its lifecycle state. Only the
// payment-domain status decision is made here; the transport adapter never
// interprets transaction state.
//
// The HighLevel Custom Payment Provider contract sends a verification payload
// containing transactionId, chargeId, and subscriptionId. RVPay only supports
// one-time payments, so a non-empty subscriptionId is rejected. The deposit
// is resolved by transactionId first; if that does not resolve, the chargeId
// is used as a fallback. The authoritative transaction state (deposit status)
// determines the verification result; success is never inferred merely from
// the existence of a chargeId or a received webhook.
func (s *Impl) VerifyPayment(ctx context.Context, req *transactionsgrpc.VerifyPaymentRequest) (*transactionsgrpc.VerifyPaymentResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "verify payment request is required")
	}

	ghlTransactionID := strings.TrimSpace(req.GetGhlTransactionId())
	ghlChargeID := strings.TrimSpace(req.GetGhlChargeId())
	subscriptionID := strings.TrimSpace(req.GetSubscriptionId())

	// RVPay only supports one-time payments. A non-empty subscriptionId
	// indicates a subscription-scoped verification, which is not supported.
	if subscriptionID != "" {
		return nil, status.Error(codes.FailedPrecondition, "subscription payments are not supported")
	}

	if ghlTransactionID == "" && ghlChargeID == "" {
		return nil, status.Error(codes.InvalidArgument, "ghl_transaction_id or ghl_charge_id is required")
	}

	deposit, err := s.resolveDeposit(ctx, ghlTransactionID, ghlChargeID)
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrNotFound):
			return nil, status.Error(codes.NotFound, "transaction not found")
		default:
			s.logger.Error().Err(err).
				Str("ghl_transaction_id", ghlTransactionID).
				Str("ghl_charge_id", ghlChargeID).
				Msg("could not verify payment")
			return nil, status.Error(codes.Internal, "could not verify payment")
		}
	}

	// Interpret the deposit lifecycle state. Only the payment domain decides
	// what COMPLETED/FAILED/INITIATED/PROCESSING mean for the provider contract.
	switch deposit.Status {
	case sqlc.DepositStatusCOMPLETED:
		return &transactionsgrpc.VerifyPaymentResponse{Success: true}, nil
	case sqlc.DepositStatusFAILED:
		return &transactionsgrpc.VerifyPaymentResponse{Failed: true}, nil
	default:
		// INITIATED and PROCESSING are pending.
		return &transactionsgrpc.VerifyPaymentResponse{}, nil
	}
}

// resolveDeposit resolves a deposit by its GoHighLevel transaction identifier
// first, then by its charge identifier as a fallback. The transaction
// identifier is the primary correlation key; the charge identifier is used
// when the transaction identifier is absent or does not resolve.
func (s *Impl) resolveDeposit(ctx context.Context, ghlTransactionID, ghlChargeID string) (sqlc.Deposit, error) {
	if ghlTransactionID != "" {
		deposit, err := s.depositRepo.GetByGHLTransactionID(ctx, ghlTransactionID)
		if err == nil {
			return deposit, nil
		}
		if !errors.Is(err, repo.ErrNotFound) {
			return sqlc.Deposit{}, err
		}
		// Transaction ID not found; fall through to charge ID lookup.
	}

	if ghlChargeID != "" {
		deposit, err := s.depositRepo.GetByGHLChargeID(ctx, ghlChargeID)
		if err == nil {
			return deposit, nil
		}
		if !errors.Is(err, repo.ErrNotFound) {
			return sqlc.Deposit{}, err
		}
	}

	return sqlc.Deposit{}, repo.ErrNotFound
}

// pawapayTerminalStatuses are deposit lifecycle states that must never be
// changed by a later callback: a completed or failed payment is final.
func pawapayTerminalStatuses() map[sqlc.DepositStatus]bool {
	return map[sqlc.DepositStatus]bool{
		sqlc.DepositStatusCOMPLETED: true,
		sqlc.DepositStatusFAILED:    true,
	}
}

// ProcessDepositCallback processes an inbound PawaPay V2 Deposit Status
// Callback. The callback is correlated strictly by deposit_id — the RVPay
// deposit UUID originally supplied to PawaPay — never by phone number,
// amount, or HighLevel/customer/merchant identifiers.
//
// Behavior:
//   - COMPLETED  → deposit marked COMPLETED (completed_at set), PawaPay
//     providerTransactionId preserved in external_reference.
//   - FAILED     → deposit marked FAILED (failed_at set) with the PawaPay
//     failure message preserved in failure_reason.
//   - PROCESSING → deposit moved to PROCESSING (no terminal transition).
//   - Unknown depositId / conflicting late callbacks are handled safely and
//     acknowledged with success so PawaPay does not retry pointlessly.
//   - Duplicate callbacks are idempotent: a terminal deposit is never
//     downgraded or re-side-effected.
func (s *Impl) ProcessDepositCallback(ctx context.Context, req *transactionsgrpc.ProcessDepositCallbackRequest) (*transactionsgrpc.ProcessDepositCallbackResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "callback request is required")
	}

	depositID := strings.TrimSpace(req.GetDepositId())
	callbackStatus := strings.TrimSpace(req.GetStatus())
	if depositID == "" || callbackStatus == "" {
		return nil, status.Error(codes.InvalidArgument, "deposit_id and status are required")
	}

	depositUUID, err := uuid.Parse(depositID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "deposit_id must be a valid deposit identifier")
	}

	s.logger.Info().
		Str("deposit_id", depositID).
		Str("status", callbackStatus).
		Str("provider_transaction_id", req.GetProviderTransactionId()).
		Msg("PawaPay deposit callback received")

	var target sqlc.DepositStatus
	switch callbackStatus {
	case "COMPLETED":
		target = sqlc.DepositStatusCOMPLETED
	case "FAILED":
		target = sqlc.DepositStatusFAILED
	case "PROCESSING":
		target = sqlc.DepositStatusPROCESSING
	default:
		s.logger.Warn().
			Str("deposit_id", depositID).
			Str("status", callbackStatus).
			Msg("unsupported PawaPay callback status")
		return nil, status.Error(codes.InvalidArgument, "unsupported callback status")
	}

	deposit, err := s.depositRepo.GetByID(ctx, depositUUID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			// Unknown depositId: acknowledge so PawaPay does not retry; there
			// is nothing this service can do for a deposit it does not have.
			s.logger.Warn().
				Str("deposit_id", depositID).
				Str("status", callbackStatus).
				Msg("PawaPay callback references unknown deposit")
			return &transactionsgrpc.ProcessDepositCallbackResponse{}, nil
		}
		s.logger.Error().Err(err).Str("deposit_id", depositID).Msg("could not look up deposit for PawaPay callback")
		return nil, status.Error(codes.Internal, "could not process callback")
	}

	// Terminal-state protection: COMPLETED and FAILED are final. Duplicate
	// callbacks are acknowledged idempotently; conflicting late callbacks are
	// logged and ignored so a terminal state can never be downgraded.
	if pawapayTerminalStatuses()[deposit.Status] {
		if deposit.Status == target {
			s.logger.Info().
				Str("deposit_id", depositID).
				Str("status", callbackStatus).
				Msg("duplicate PawaPay callback for terminal deposit; acknowledged idempotently")
			return &transactionsgrpc.ProcessDepositCallbackResponse{}, nil
		}
		s.logger.Warn().
			Str("deposit_id", depositID).
			Str("current_status", string(deposit.Status)).
			Str("status", callbackStatus).
			Msg("conflicting PawaPay callback for terminal deposit; ignored")
		return &transactionsgrpc.ProcessDepositCallbackResponse{}, nil
	}

	return s.applyPawaPayCallbackTransition(ctx, depositID, target, req)
}

// applyPawaPayCallbackTransition applies a validated, non-terminal callback
// status to the deposit and records the PawaPay provider reference. The
// deposit finalization and the GHL synchronization intent (durable outbox)
// are committed in a single database transaction; the external GHL call is
// never made here — a separate worker performs it after commit.
func (s *Impl) applyPawaPayCallbackTransition(ctx context.Context, depositID string, target sqlc.DepositStatus, req *transactionsgrpc.ProcessDepositCallbackRequest) (*transactionsgrpc.ProcessDepositCallbackResponse, error) {
	depositUUID, err := uuid.Parse(depositID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "deposit_id must be a valid deposit identifier")
	}

	txQuerier, tx, err := s.transactionsRepo.Begin(ctx)
	if err != nil {
		s.logger.Error().Err(err).Str("deposit_id", depositID).Msg("could not begin deposit finalization transaction")
		return nil, status.Error(codes.Internal, "could not process callback")
	}
	committed := false
	defer func() {
		if !committed {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				s.logger.Error().Err(rbErr).Str("deposit_id", depositID).Msg("could not roll back deposit finalization transaction")
			}
		}
	}()

	txDepositRepo := repo.NewDepositRepo(txQuerier)

	switch target {
	case sqlc.DepositStatusPROCESSING:
		// PROCESSING is non-terminal; no final GHL synchronization is queued.
		if _, err := txDepositRepo.UpdateStatus(ctx, depositUUID, sqlc.DepositStatusPROCESSING); err != nil {
			s.logger.Error().Err(err).Str("deposit_id", depositID).Msg("could not mark deposit processing from PawaPay callback")
			return nil, status.Error(codes.Internal, "could not process callback")
		}
	case sqlc.DepositStatusCOMPLETED, sqlc.DepositStatusFAILED:
		// FinalizeDepositAndQueueGhlSync atomically sets the terminal status
		// (and completed_at / failed_at + failure_reason) AND enqueues the GHL
		// synchronization intent (ghl_sync_status='pending') when a
		// ghl_order_id is present. PROCESSING is excluded from the terminal
		// branch because it never enqueues a final update.
		failureReason := ""
		if target == sqlc.DepositStatusFAILED {
			failureReason = req.GetFailureReason().GetFailureMessage()
			if failureReason == "" {
				failureReason = req.GetFailureReason().GetFailureCode()
			}
		}
		if _, err := txDepositRepo.FinalizeDepositAndQueueGhlSync(ctx, depositUUID, target, failureReason); err != nil {
			s.logger.Error().Err(err).Str("deposit_id", depositID).Msg("could not finalize deposit from PawaPay callback")
			return nil, status.Error(codes.Internal, "could not process callback")
		}
	}

	// Preserve PawaPay's own transaction reference when provided. This never
	// touches ghl_transaction_id, which remains the HighLevel correlation ID
	// used by VerifyPayment.
	if req.GetProviderTransactionId() != "" {
		if err := txDepositRepo.SetExternalReference(ctx, depositUUID, req.GetProviderTransactionId()); err != nil {
			s.logger.Error().Err(err).Str("deposit_id", depositID).Msg("could not record PawaPay provider transaction reference")
			return nil, status.Error(codes.Internal, "could not process callback")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		s.logger.Error().Err(err).Str("deposit_id", depositID).Msg("could not commit deposit finalization transaction")
		return nil, status.Error(codes.Internal, "could not process callback")
	}
	committed = true

	s.logger.Info().
		Str("deposit_id", depositID).
		Str("status", string(target)).
		Str("provider_transaction_id", req.GetProviderTransactionId()).
		Msg("PawaPay deposit callback processed")

	return &transactionsgrpc.ProcessDepositCallbackResponse{}, nil
}

// ProcessPaymentWebhook processes a payment-provider webhook event. It
// correlates the HighLevel transaction/charge with an RVPay deposit and
// records the GHL reference on the deposit. Only one-time payment events
// relevant to RVPay are processed (payment.captured); subscription events are
// not supported and are acknowledged safely. Webhook idempotency is enforced
// by the transport adapter (Clients) via the webhook_events table; recording
// the GHL reference is naturally idempotent.
func (s *Impl) ProcessPaymentWebhook(ctx context.Context, req *transactionsgrpc.ProcessPaymentWebhookRequest) (*transactionsgrpc.ProcessPaymentWebhookResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "payment webhook request is required")
	}

	eventType := strings.TrimSpace(req.GetEventType())
	if eventType == "" {
		return nil, status.Error(codes.InvalidArgument, "event_type is required")
	}

	// Only payment.captured is relevant to the current one-time payment flow.
	// Unknown event types are acknowledged safely without processing.
	if eventType != "payment.captured" {
		s.logger.Info().Str("event_type", eventType).Msg("Unhandled payment webhook event type")
		return &transactionsgrpc.ProcessPaymentWebhookResponse{}, nil
	}

	if strings.TrimSpace(req.GetTransactionId()) == "" {
		s.logger.Warn().Str("event_type", eventType).Msg("payment.captured event missing transaction_id")
		return &transactionsgrpc.ProcessPaymentWebhookResponse{}, nil
	}

	// Correlate the HighLevel transaction with an RVPay deposit. If the
	// deposit is not found, acknowledge safely; the event is already recorded
	// for idempotency by the transport adapter and a later reconciliation can
	// resolve it.
	deposit, err := s.depositRepo.GetByGHLTransactionID(ctx, req.GetTransactionId())
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			s.logger.Warn().Str("transaction_id", req.GetTransactionId()).Msg("payment.captured event references unknown transaction")
			return &transactionsgrpc.ProcessPaymentWebhookResponse{}, nil
		}
		s.logger.Error().Err(err).Str("transaction_id", req.GetTransactionId()).Msg("could not correlate payment.captured event")
		return nil, status.Error(codes.Internal, "could not correlate payment event")
	}

	// Record the GHL charge reference on the deposit. The charge ID is the
	// provider's charge reference; it is correlated with the RVPay deposit so
	// charge-scoped lookups (and future reconciliation) can resolve it.
	if req.GetChargeId() != "" {
		_, err = s.depositRepo.UpdateGHLReference(ctx, deposit.ID, req.GetTransactionId(), req.GetChargeId())
		if err != nil {
			s.logger.Error().Err(err).Str("deposit_id", deposit.ID.String()).Msg("could not record GHL reference on deposit")
			return nil, status.Error(codes.Internal, "could not record payment reference")
		}
	}

	s.logger.Info().
		Str("deposit_id", deposit.ID.String()).
		Str("transaction_id", req.GetTransactionId()).
		Str("charge_id", req.GetChargeId()).
		Msg("payment.captured event processed")

	return &transactionsgrpc.ProcessPaymentWebhookResponse{}, nil
}


func (s *Impl) ProcessRefundCallback(ctx context.Context, req *transactionsgrpc.ProcessRefundCallbackRequest) (*transactionsgrpc.ProcessRefundCallbackResponse, error) {
	panic("not implemented: ProcessRefundCallback")
}

func (s *Impl) ProcessCheckoutCallback(ctx context.Context, req *transactionsgrpc.ProcessCheckoutCallbackRequest) (*transactionsgrpc.ProcessCheckoutCallbackResponse, error) {
	panic("not implemented: ProcessCheckoutCallback")
}
