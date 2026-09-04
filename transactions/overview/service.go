package overview

import (
	"context"
	"fmt"
	"strconv"
	"time"

	transactionsgrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/transactionsgrpc"
	"github.com/I-Frostbyte/rvpay-go/transactions/db/repo"
	"github.com/I-Frostbyte/rvpay-go/transactions/db/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Impl implements the DashboardOverviewService gRPC server.
type Impl struct {
	depositRepo repo.DepositRepo
	payoutRepo  repo.PayoutRepo
	logger      zerolog.Logger

	transactionsgrpc.UnimplementedDashboardOverviewServiceServer
}

// NewOverviewService creates a new overview service.
func NewOverviewService(
	depositRepo repo.DepositRepo,
	payoutRepo repo.PayoutRepo,
	logger zerolog.Logger,
) *Impl {
	return &Impl{
		depositRepo: depositRepo,
		payoutRepo:  payoutRepo,
		logger:      logger,
	}
}

// periodToSince maps a dashboard period string to a UTC start time.
func periodToSince(period string) time.Time {
	now := time.Now().UTC()
	switch period {
	case "30d":
		return now.AddDate(0, 0, -30)
	case "90d":
		return now.AddDate(0, 0, -90)
	case "ytd":
		return time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	default:
		return now.AddDate(0, 0, -7)
	}
}

// GetOverviewSnapshot aggregates the metrics rendered on the Admin Dashboard
// overview page for the requested period. Deposits drive revenue + volume;
// payouts drive recent-payout rows. active_sub_accounts and needs_attention
// are not populated here: see the response field comments and
// dashboard-setup.md for the cross-service gaps.
func (s *Impl) GetOverviewSnapshot(ctx context.Context, req *transactionsgrpc.GetOverviewSnapshotRequest) (*transactionsgrpc.GetOverviewSnapshotResponse, error) {
	if req == nil {
		req = &transactionsgrpc.GetOverviewSnapshotRequest{}
	}
	since := periodToSince(req.GetPeriod())

	revenue, err := s.depositRepo.SumAmountInWindow(ctx, since)
	if err != nil {
		s.logger.Error().Err(err).Msg("could not sum deposit revenue")
		return nil, status.Error(codes.Internal, "could not load overview snapshot")
	}
	volume, err := s.depositRepo.CountInWindow(ctx, since)
	if err != nil {
		s.logger.Error().Err(err).Msg("could not count deposit volume")
		return nil, status.Error(codes.Internal, "could not load overview snapshot")
	}
	buckets, err := s.depositRepo.RevenueOverTimeInWindow(ctx, since)
	if err != nil {
		s.logger.Error().Err(err).Msg("could not load revenue over time")
		return nil, status.Error(codes.Internal, "could not load overview snapshot")
	}
	recentPayouts, err := s.payoutRepo.ListFiltered(ctx, "", "", 5, 0)
	if err != nil {
		s.logger.Error().Err(err).Msg("could not load recent payouts")
		return nil, status.Error(codes.Internal, "could not load overview snapshot")
	}

	revenueF, _ := revenue.Float64Value()

	revenueOverTime := make([]*transactionsgrpc.RevenueBucket, 0, len(buckets))
	for i, bucket := range buckets {
		rev := revenueFromInterface(bucket.Revenue)
		label := bucket.PeriodLabel
		if label == "" {
			label = bucketLabel(i)
		}
		revenueOverTime = append(revenueOverTime, &transactionsgrpc.RevenueBucket{
			PeriodLabel: label,
			Revenue:     rev,
		})
	}

	rows := make([]*transactionsgrpc.OverviewPayoutRow, 0, len(recentPayouts))
	for i, payout := range recentPayouts {
		if i >= 5 {
			break
		}
		rows = append(rows, &transactionsgrpc.OverviewPayoutRow{
			SubAccount: textOrDash(payout.DestinationReference),
			Amount:     formatOverviewAmount(payout.Amount, payout.Currency),
			Status:     overviewPayoutStatus(payout.Status),
			Date:       formatOverviewTime(payout.RequestedAt),
		})
	}

	return &transactionsgrpc.GetOverviewSnapshotResponse{
		TotalRevenue:      int64(revenueF.Float64),
		RevenueCurrency:   "XAF",
		TransactionVolume: volume,
		// active_sub_accounts: deliberately 0 (cross-service gap).
		ActiveSubAccounts: 0,
		// pending_payouts: 0 here; the payouts overview stats endpoint owns
		// accurate pending derivation. Left 0 to avoid double-counting.
		PendingPayouts:   0,
		NeedsAttention:   nil,
		RevenueOverTime:  revenueOverTime,
		RecentPayouts:    rows,
	}, nil
}

func textOrDash(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}

func revenueFromInterface(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	default:
		var num pgtype.Numeric
		_ = num.Scan(v)
		f, _ := num.Float64Value()
		return int64(f.Float64)
	}
}

func formatOverviewAmount(amount pgtype.Numeric, currency string) string {
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

func overviewPayoutStatus(status sqlc.PayoutStatus) string {
	switch status {
	case sqlc.PayoutStatusREQUESTED, sqlc.PayoutStatusPROCESSING:
		return "Processing"
	case sqlc.PayoutStatusCOMPLETED:
		return "Paid"
	case sqlc.PayoutStatusFAILED:
		return "Failed"
	default:
		return "Processing"
	}
}

func formatOverviewTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("Jan 02, 2006")
}

func bucketLabel(i int) string {
	return fmt.Sprintf("Day %d", i+1)
}
