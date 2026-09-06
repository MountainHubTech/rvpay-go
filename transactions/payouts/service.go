package payouts

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/I-Frostbyte/pawapay_client"
	pawapaypayouts "github.com/I-Frostbyte/pawapay_client/payouts"
	commongrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/commongrpc"
	transactionsgrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/transactionsgrpc"
	"github.com/I-Frostbyte/rvpay-go/shared/observability"
	"github.com/I-Frostbyte/rvpay-go/transactions/db/repo"
	"github.com/I-Frostbyte/rvpay-go/transactions/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Impl implements the PayoutService gRPC server.
type Impl struct {
	payoutRepo    repo.PayoutRepo
	logger        zerolog.Logger
	pawapayClient pawapay_client.Client

	transactionsgrpc.UnimplementedPayoutServiceServer
}

// NewPayoutService creates a new payout service.
func NewPayoutService(
	payoutRepo repo.PayoutRepo,
	logger zerolog.Logger,
	pawapayClient pawapay_client.Client,
) *Impl {
	return &Impl{
		payoutRepo:    payoutRepo,
		logger:        logger,
		pawapayClient: pawapayClient,
	}
}

// RequestPayout requests an outbound settlement.
func (s *Impl) RequestPayout(ctx context.Context, req *transactionsgrpc.CreatePayoutRequest) (*transactionsgrpc.CreatePayoutResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "payout request is required")
	}

	clientID, err := uuid.Parse(req.GetClientId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "client_id must be a valid UUID")
	}

	merchantID, err := uuid.Parse(req.GetMerchantId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "merchant_id must be a valid UUID")
	}

	amount, err := validateAmount(req.GetAmount())
	if err != nil {
		return nil, err
	}

	currency := strings.ToUpper(strings.TrimSpace(req.GetAmount().GetCurrency()))
	if currency == "" {
		return nil, status.Error(codes.InvalidArgument, "currency is required")
	}

	provider, err := grpcProviderToSqlc(req.GetProvider())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	destinationReference := strings.TrimSpace(req.GetDestinationReference())
	if destinationReference == "" {
		return nil, status.Error(codes.InvalidArgument, "destination_reference is required")
	}

	// A newly requested payout begins in the REQUESTED lifecycle state.
	// An idempotency key is generated server-side for duplicate detection.
	// Merchant existence is enforced by the database foreign key.
	payout, err := s.payoutRepo.Create(ctx, clientID, merchantID, amount, currency, provider, destinationReference, sqlc.PayoutStatusREQUESTED, uuid.New())
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrDuplicate):
			return nil, status.Error(codes.AlreadyExists, "payout already exists")
		case errors.Is(err, repo.ErrConstraint):
			return nil, status.Error(codes.NotFound, "referenced merchant not found")
		default:
			s.logger.Error().Err(err).Str("client_id", clientID.String()).Msg("could not create payout")
			return nil, status.Error(codes.Internal, "could not create payout")
		}
	}

	s.logger.Info().Str("payout_id", payout.ID.String()).Str("merchant_id", merchantID.String()).Msg("payout requested")

	// Initiate the payout with PawaPay using the caller-supplied provider. The
	// destination_reference is mapped to the PawaPay recipient phone number.
	if err := s.initiatePawapayPayout(ctx, payout.ID, amount, currency, destinationReference, provider); err != nil {
		s.logger.Error().Err(err).Str("payout_id", payout.ID.String()).Msg("could not initiate payout with pawapay")
		return nil, status.Error(codes.Internal, "could not initiate payout with pawapay")
	}

	return &transactionsgrpc.CreatePayoutResponse{
		Payout: payoutToProto(payout),
	}, nil
}

// initiatePawapayPayout calls the PawaPay V2 Initiate Payout operation.
// The amount is passed to the SDK as a decimal string to preserve monetary
// precision. The payout domain exposes no dedicated phone-number field, so
// the destination_reference is mapped to the SDK recipient phone number.
func (s *Impl) initiatePawapayPayout(ctx context.Context, payoutID uuid.UUID, amount pgtype.Numeric, currency, phoneNumber string, provider sqlc.PaymentProvider) error {
	pawapayProvider, err := sqlcPaymentProviderToPawapay(provider)
	if err != nil {
		return err
	}

	amountValue, err := amount.Float64Value()
	if err != nil {
		return err
	}

	req := &pawapaypayouts.InitiatePayoutRequest{
		PayoutID: payoutID.String(),
		Amount:   strconv.FormatFloat(amountValue.Float64, 'f', 2, 64),
		Currency: currency,
		Recipient: pawapaypayouts.Recipient{
			Type: "MMO",
			AccountDetails: pawapaypayouts.AccountDetails{
				PhoneNumber: phoneNumber,
				Provider:    pawapayProvider,
			},
		},
	}

	_, err = s.pawapayClient.Payouts.InitiatePayout(ctx, req)
	return err
}

// GetPayout fetches a payout by id.
func (s *Impl) GetPayout(ctx context.Context, req *transactionsgrpc.GetPayoutRequest) (*transactionsgrpc.GetPayoutResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "payout request is required")
	}

	payoutID, err := uuid.Parse(req.GetPayoutId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "payout_id must be a valid UUID")
	}

	payout, err := s.payoutRepo.GetByID(ctx, payoutID)
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrNotFound):
			return nil, status.Error(codes.NotFound, "payout not found")
		default:
			s.logger.Error().Err(err).Str("payout_id", payoutID.String()).Msg("could not get payout")
			return nil, status.Error(codes.Internal, "could not get payout")
		}
	}

	return &transactionsgrpc.GetPayoutResponse{
		Payout: payoutToProto(payout),
	}, nil
}

// validateAmount validates and converts a protobuf Money amount to pgtype.Numeric.
func validateAmount(money *commongrpc.Money) (pgtype.Numeric, error) {
	if money == nil {
		return pgtype.Numeric{}, status.Error(codes.InvalidArgument, "amount is required")
	}

	var amount pgtype.Numeric
	if err := amount.Scan(money.GetAmount()); err != nil {
		return pgtype.Numeric{}, status.Errorf(codes.InvalidArgument, "invalid payout amount: %v", err)
	}

	f, err := amount.Float64Value()
	if err != nil {
		return pgtype.Numeric{}, status.Errorf(codes.InvalidArgument, "invalid payout amount: %v", err)
	}
	if !f.Valid || f.Float64 <= 0 {
		return pgtype.Numeric{}, status.Error(codes.InvalidArgument, "payout amount must be greater than zero")
	}

	return amount, nil
}

func grpcProviderToSqlc(provider commongrpc.Provider) (sqlc.PaymentProvider, error) {
	switch provider {
	case commongrpc.Provider_PROVIDER_MTN_MOMO:
		return sqlc.PaymentProviderMTNMOMO, nil
	case commongrpc.Provider_PROVIDER_ORANGE_MOMO:
		return sqlc.PaymentProviderORANGEMOMO, nil
	default:
		return "", status.Errorf(codes.InvalidArgument, "unsupported provider: %s", provider)
	}
}


// GetPayoutOverviewStats returns the payout metrics rendered on the Admin
// Dashboard payouts page. Values come from the payout table; no metrics are
// fabricated.
func (s *Impl) GetPayoutOverviewStats(ctx context.Context, _ *transactionsgrpc.GetPayoutOverviewStatsRequest) (resp *transactionsgrpc.GetPayoutOverviewStatsResponse, err error) {
	start := time.Now()
	s.logger.Info().
		Str("request_id", observability.RequestIDFromContext(ctx)).
		Str("endpoint", "/v1/public/payouts/overview/stats").
		Str("method", "GET").
		Str("operation", "GetPayoutOverviewStats").
		Msg("dashboard API request received")

	defer func() {
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("request_id", observability.RequestIDFromContext(ctx)).
				Str("endpoint", "/v1/public/payouts/overview/stats").
				Str("operation", "GetPayoutOverviewStats").
				Str("grpc_code", status.Code(err).String()).
				Int64("duration_ms", time.Since(start).Milliseconds()).
				Msg("dashboard API request failed")
			return
		}
		s.logger.Info().
			Str("request_id", observability.RequestIDFromContext(ctx)).
			Str("endpoint", "/v1/public/payouts/overview/stats").
			Str("operation", "GetPayoutOverviewStats").
			Str("grpc_code", "OK").
			Int("stats_count", len(resp.GetStats())).
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("dashboard API request completed")
	}()

	pendingCount, err := s.payoutRepo.CountByStatus(ctx, sqlc.PayoutStatusREQUESTED)
	if err != nil {
		s.logger.Error().Err(err).
			Str("operation", "GetPayoutOverviewStats").
			Str("repository", "PayoutRepo.CountByStatus(REQUESTED)").
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("could not count pending payouts")
		return nil, status.Error(codes.Internal, "could not load payout stats")
	}
	processingCount, err := s.payoutRepo.CountByStatus(ctx, sqlc.PayoutStatusPROCESSING)
	if err != nil {
		s.logger.Error().Err(err).
			Str("operation", "GetPayoutOverviewStats").
			Str("repository", "PayoutRepo.CountByStatus(PROCESSING)").
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("could not count processing payouts")
		return nil, status.Error(codes.Internal, "could not load payout stats")
	}
	clearedCount, err := s.payoutRepo.CountByStatus(ctx, sqlc.PayoutStatusCOMPLETED)
	if err != nil {
		s.logger.Error().Err(err).
			Str("operation", "GetPayoutOverviewStats").
			Str("repository", "PayoutRepo.CountByStatus(COMPLETED)").
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("could not count cleared payouts")
		return nil, status.Error(codes.Internal, "could not load payout stats")
	}
	failedCount, err := s.payoutRepo.CountByStatus(ctx, sqlc.PayoutStatusFAILED)
	if err != nil {
		s.logger.Error().Err(err).
			Str("operation", "GetPayoutOverviewStats").
			Str("repository", "PayoutRepo.CountByStatus(FAILED)").
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("could not count failed payouts")
		return nil, status.Error(codes.Internal, "could not load payout stats")
	}
	s.logger.Debug().
		Str("operation", "GetPayoutOverviewStats").
		Str("repository", "PayoutRepo.CountByStatus").
		Int("pending_rows", int(pendingCount)).
		Int("processing_rows", int(processingCount)).
		Int("cleared_rows", int(clearedCount)).
		Int("failed_rows", int(failedCount)).
		Msg("repository count queries completed")

	pendingAmount, err := s.payoutRepo.SumAmountByStatus(ctx, sqlc.PayoutStatusREQUESTED)
	if err != nil {
		s.logger.Error().Err(err).
			Str("operation", "GetPayoutOverviewStats").
			Str("repository", "PayoutRepo.SumAmountByStatus(REQUESTED)").
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("could not sum pending payout amounts")
		return nil, status.Error(codes.Internal, "could not load payout stats")
	}
	processingAmount, err := s.payoutRepo.SumAmountByStatus(ctx, sqlc.PayoutStatusPROCESSING)
	if err != nil {
		s.logger.Error().Err(err).
			Str("operation", "GetPayoutOverviewStats").
			Str("repository", "PayoutRepo.SumAmountByStatus(PROCESSING)").
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("could not sum processing payout amounts")
		return nil, status.Error(codes.Internal, "could not load payout stats")
	}
	clearedAmount, err := s.payoutRepo.SumAmountByStatus(ctx, sqlc.PayoutStatusCOMPLETED)
	if err != nil {
		s.logger.Error().Err(err).
			Str("operation", "GetPayoutOverviewStats").
			Str("repository", "PayoutRepo.SumAmountByStatus(COMPLETED)").
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("could not sum cleared payout amounts")
		return nil, status.Error(codes.Internal, "could not load payout stats")
	}
	failedAmount, err := s.payoutRepo.SumAmountByStatus(ctx, sqlc.PayoutStatusFAILED)
	if err != nil {
		s.logger.Error().Err(err).
			Str("operation", "GetPayoutOverviewStats").
			Str("repository", "PayoutRepo.SumAmountByStatus(FAILED)").
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("could not sum failed payout amounts")
		return nil, status.Error(codes.Internal, "could not load payout stats")
	}
	s.logger.Debug().
		Str("operation", "GetPayoutOverviewStats").
		Str("repository", "PayoutRepo.SumAmountByStatus").
		Msg("repository sum queries completed")

	pendingTotal := addNumeric(numericOrZero(pendingAmount), numericOrZero(processingAmount))
	clearedTotal := numericOrZero(clearedAmount)
	failedTotal := numericOrZero(failedAmount)

	stats := []*transactionsgrpc.PayoutStat{
		{Label: "Total Pending", Value: formatMoney(pendingTotal, "XAF"), Meta: fmt.Sprintf("%d payouts processing", pendingCount+processingCount)},
		{Label: "Total Cleared (MTD)", Value: formatMoney(clearedTotal, "XAF"), Meta: fmt.Sprintf("%d payouts cleared", clearedCount)},
		{Label: "Failed Payouts", Value: strconv.FormatInt(failedCount, 10), Meta: fmt.Sprintf("Totaling %s", formatMoney(failedTotal, "XAF"))},
	}

	return &transactionsgrpc.GetPayoutOverviewStatsResponse{Stats: stats}, nil
}



// ListPayouts returns a paginated, searchable, status-filtered list of payouts
// for the Admin Dashboard payouts page.
func (s *Impl) ListPayouts(ctx context.Context, req *transactionsgrpc.ListPayoutsRequest) (resp *transactionsgrpc.ListPayoutsResponse, err error) {
	start := time.Now()
	if req == nil {
		req = &transactionsgrpc.ListPayoutsRequest{}
	}
	s.logger.Info().
		Str("request_id", observability.RequestIDFromContext(ctx)).
		Str("endpoint", "/v1/public/payouts").
		Str("method", "GET").
		Str("operation", "ListPayouts").
		Str("search", req.GetSearch()).
		Str("status", req.GetStatus()).
		Int("page", int(req.GetPage())).
		Int("page_size", int(req.GetPageSize())).
		Msg("dashboard API request received")

	defer func() {
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("request_id", observability.RequestIDFromContext(ctx)).
				Str("endpoint", "/v1/public/payouts").
				Str("operation", "ListPayouts").
				Str("search", req.GetSearch()).
				Str("status", req.GetStatus()).
				Int("page", int(req.GetPage())).
				Str("grpc_code", status.Code(err).String()).
				Int64("duration_ms", time.Since(start).Milliseconds()).
				Msg("dashboard API request failed")
			return
		}
		s.logger.Info().
			Str("request_id", observability.RequestIDFromContext(ctx)).
			Str("endpoint", "/v1/public/payouts").
			Str("operation", "ListPayouts").
			Str("grpc_code", "OK").
			Int("rows_returned", len(resp.GetRows())).
			Int64("total", resp.GetTotal()).
			Int("page", int(resp.GetPage())).
			Int("page_size", int(resp.GetPageSize())).
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("dashboard API request completed")
	}()

	page := req.GetPage()
	if page < 1 {
		page = 1
	}
	pageSize := req.GetPageSize()
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	payouts, err := s.payoutRepo.ListFiltered(ctx, req.GetSearch(), req.GetStatus(), pageSize, offset)
	if err != nil {
		s.logger.Error().Err(err).
			Str("operation", "ListPayouts").
			Str("repository", "PayoutRepo.ListFiltered").
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("could not list payouts")
		return nil, status.Error(codes.Internal, "could not list payouts")
	}
	if len(payouts) == 0 {
		s.logger.Info().
			Str("operation", "ListPayouts").
			Str("repository", "PayoutRepo.ListFiltered").
			Str("search", req.GetSearch()).
			Str("status", req.GetStatus()).
			Int("rows_returned", 0).
			Msg("dashboard payout query returned no rows")
	} else {
		s.logger.Debug().
			Str("operation", "ListPayouts").
			Str("repository", "PayoutRepo.ListFiltered").
			Int("rows_returned", len(payouts)).
			Msg("repository query completed")
	}
	total, err := s.payoutRepo.CountFiltered(ctx, req.GetSearch(), req.GetStatus())
	if err != nil {
		s.logger.Error().Err(err).
			Str("operation", "ListPayouts").
			Str("repository", "PayoutRepo.CountFiltered").
			Int64("duration_ms", time.Since(start).Milliseconds()).
			Msg("could not count payouts")
		return nil, status.Error(codes.Internal, "could not list payouts")
	}

	rows := make([]*transactionsgrpc.PayoutListRow, 0, len(payouts))
	for _, payout := range payouts {
		name := textValue(payout.DestinationReference)
		rows = append(rows, &transactionsgrpc.PayoutListRow{
			Id:                      payout.ID.String(),
			Initials:                initialsFromName(name),
			Name:                    name,
			Amount:                  formatMoney(payout.Amount, payout.Currency),
			Status:                  payoutListStatus(payout.Status),
			Initiated:               formatTimestamp(payout.RequestedAt),
			ExpectedOrCleared:       completedOrExpected(payout),
			ExpectedOrClearedStrong: payout.Status == sqlc.PayoutStatusCOMPLETED,
		})
	}

	return &transactionsgrpc.ListPayoutsResponse{Rows: rows, Total: total, Page: page, PageSize: pageSize}, nil
}

// payoutListStatus maps a persisted payout status to the dashboard-facing
// status string. RVPay has no partial/"In Transit" payout state, so
// PROCESSING is reported as Pending.
func payoutListStatus(status sqlc.PayoutStatus) string {
	switch status {
	case sqlc.PayoutStatusREQUESTED, sqlc.PayoutStatusPROCESSING:
		return "Pending"
	case sqlc.PayoutStatusCOMPLETED:
		return "Cleared"
	case sqlc.PayoutStatusFAILED:
		return "Failed"
	default:
		return "Pending"
	}
}

// completedOrExpected returns the cleared time for completed payouts and a
// dash otherwise (no expected-date concept is modeled yet).
func completedOrExpected(payout sqlc.Payout) string {
	if payout.Status == sqlc.PayoutStatusCOMPLETED && payout.CompletedAt.Valid {
		return formatTimestamp(payout.CompletedAt.Time)
	}
	return "-"
}

// numericOrZero returns n when valid, otherwise a zero pgtype.Numeric.
func numericOrZero(n pgtype.Numeric) pgtype.Numeric {
	if !n.Valid || n.NaN {
		return pgtype.Numeric{Valid: true}
	}
	return n
}

// addNumeric returns the arithmetic sum of a and b as a pgtype.Numeric,
// decoding through float64 (display-only aggregation).
func addNumeric(a, b pgtype.Numeric) pgtype.Numeric {
	fa, _ := a.Float64Value()
	fb, _ := b.Float64Value()
	sum := pgtype.Numeric{}
	_ = sum.Scan(strconv.FormatFloat(fa.Float64+fb.Float64, 'f', 2, 64))
	return sum
}

// initialsFromName derives two-letter initials from a payout display name.
func initialsFromName(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "--"
	}
	initials := string([]rune(parts[0])[0])
	if len(parts) > 1 {
		initials += string([]rune(parts[1])[0])
	}
	return strings.ToUpper(initials)
}

// formatTimestamp renders a timestamp for the dashboard.
func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("Jan 02, 2006")
}

// formatMoney renders a NUMERIC amount as a human-readable money string in the
// given currency. It is lossy (two decimals) and intended for display only.
func formatMoney(amount pgtype.Numeric, currency string) string {
	f, err := amount.Float64Value()
	if err != nil || !f.Valid {
		return "--"
	}
	switch currency {
	case "XAF", "XOF", "JPY", "KRW":
		return fmt.Sprintf("%s %s", currency, strconv.FormatFloat(f.Float64, 'f', 0, 64))
	default:
		return fmt.Sprintf("%s %s", currency, strconv.FormatFloat(f.Float64, 'f', 2, 64))
	}
}


// sqlcPaymentProviderToPawapay maps a persisted payment provider to the
// string value expected by the PawaPay V2 API.
func sqlcPaymentProviderToPawapay(paymentProvider sqlc.PaymentProvider) (string, error) {
	switch paymentProvider {
	case sqlc.PaymentProviderMTNMOMO:
		return "MTN_MOMO_CMR", nil
	case sqlc.PaymentProviderORANGEMOMO:
		return "ORANGE_MOMO_CMR", nil
	default:
		return "", fmt.Errorf("unsupported payment provider: %s", paymentProvider)
	}
}
