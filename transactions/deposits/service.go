package deposits

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/I-Frostbyte/pawapay_client"
	pawapaydeposits "github.com/I-Frostbyte/pawapay_client/deposits"
	commongrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/commongrpc"
	transactionsgrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/transactionsgrpc"
	"github.com/I-Frostbyte/rvpay-go/transactions/db/repo"
	"github.com/I-Frostbyte/rvpay-go/transactions/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Impl implements the DepositService gRPC server.
type Impl struct {
	depositRepo      repo.DepositRepo
	transactionsRepo repo.TransactionsRepo
	logger           zerolog.Logger
	pawapayClient    pawapay_client.Client

	transactionsgrpc.UnimplementedDepositServiceServer
}

// NewDepositService creates a new deposit service.
func NewDepositService(
	depositRepo repo.DepositRepo,
	transactionsRepo repo.TransactionsRepo,
	logger zerolog.Logger,
	pawapayClient pawapay_client.Client,
) *Impl {
	return &Impl{
		depositRepo:      depositRepo,
		transactionsRepo: transactionsRepo,
		logger:           logger,
		pawapayClient:    pawapayClient,
	}
}

// InitiateDeposit initiates a customer deposit.
func (s *Impl) InitiateDeposit(ctx context.Context, req *transactionsgrpc.CreateDepositRequest) (*transactionsgrpc.CreateDepositResponse, error) {
	s.logger.Info().Msg("Initializing InitiateDeposit...")

	s.logger.Info().Msgf("Initate Deposit request body: %v", req)

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "deposit request is required")
	}

	clientName := strings.TrimSpace(req.GetClientName())
	if clientName == "" {
		return nil, status.Error(codes.InvalidArgument, "client_name is required")
	}

	// customer_id and merchant_id are external HighLevel identifiers
	// (contact.id and transactionId respectively). They are optional and
	// must NOT be parsed as UUIDs.
	customerID := strings.TrimSpace(req.GetCustomerId())
	// merchant_id temporarily carries the payer phone number per the current
	// HighLevel mapping requirement; it is NOT the HighLevel transaction ID.
	merchantID := strings.TrimSpace(req.GetMerchantId())
	// The HighLevel transaction ID is persisted in deposits.ghl_transaction_id
	// and is what the verify endpoint resolves deposits by. It must NOT be
	// stored in merchant_id.
	ghlTransactionID := strings.TrimSpace(req.GetGhlTransactionId())

	amount, err := validateAmount(req.GetAmount())
	if err != nil {
		return nil, err
	}

	currency := strings.ToUpper(strings.TrimSpace(req.GetAmount().GetCurrency()))
	if currency == "" {
		return nil, status.Error(codes.InvalidArgument, "currency is required")
	}

	paymentType, err := grpcPaymentTypeToSqlc(req.GetPaymentType())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	phoneNumber := strings.TrimSpace(req.GetPayerPhoneNumber())
	if phoneNumber == "" {
		return nil, status.Error(codes.InvalidArgument, "payer_phone_number is required")
	}

	provider, err := grpcProviderToSqlc(req.GetProvider())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// The deposit creation and the PawaPay initiation are made consistent
	// with a single simple database transaction: if the deposit INSERT
	// succeeds but PawaPay initiation fails, the INSERT is rolled back so
	// no orphaned deposit remains.
	txQuerier, tx, err := s.transactionsRepo.Begin(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("could not begin deposit database transaction")
		return nil, status.Error(codes.Internal, "could not create deposit")
	}
	committed := false
	defer func() {
		if !committed {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				s.logger.Error().Err(rbErr).Msg("could not roll back deposit database transaction")
			}
		}
	}()

	deposit, err := repo.NewDepositRepo(txQuerier).Create(ctx, clientName, customerID, merchantID, amount, currency, paymentType, phoneNumber, provider, sqlc.DepositStatusINITIATED, uuid.New(), ghlTransactionID)
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrDuplicate):
			return nil, status.Error(codes.AlreadyExists, "deposit already exists")
		default:
			s.logger.Error().Err(err).Str("client_name", clientName).Msg("could not create deposit")
			return nil, status.Error(codes.Internal, "could not create deposit")
		}
	}

	s.logger.Info().Str("deposit_id", deposit.ID.String()).Str("merchant_id", merchantID).Msg("deposit initiated")

	// Initiate the deposit with PawaPay using the caller-supplied provider and
	// payer phone number. The deposit is persisted in the INITIATED lifecycle
	// state within the open transaction; the PawaPay request is the external
	// initiation step. On failure the deferred rollback undoes the INSERT.
	if err := s.initiatePawapayDeposit(ctx, deposit.ID, amount, currency, phoneNumber, provider); err != nil {
		s.logger.Error().Err(err).Str("deposit_id", deposit.ID.String()).Msg("could not initiate deposit with pawapay")
		return nil, status.Error(codes.Internal, "could not initiate deposit with pawapay")
	}

	if err := tx.Commit(ctx); err != nil {
		s.logger.Error().Err(err).Str("deposit_id", deposit.ID.String()).Msg("could not commit deposit database transaction")
		return nil, status.Error(codes.Internal, "could not create deposit")
	}
	committed = true

	return &transactionsgrpc.CreateDepositResponse{
		Deposit: depositToProto(deposit),
	}, nil
}

// initiatePawapayDeposit calls the PawaPay V2 Initiate Deposit operation.
// The amount is passed to the SDK as a decimal string to preserve monetary
// precision.
func (s *Impl) initiatePawapayDeposit(ctx context.Context, depositID uuid.UUID, amount pgtype.Numeric, currency, phoneNumber string, provider sqlc.PaymentProvider) error {
	s.logger.Info().Msg("Initiating deposit with PawaPay...")

	pawapayProvider, err := sqlcPaymentProviderToPawapay(provider)
	if err != nil {
		return err
	}

	amountValue, err := amount.Float64Value()
	if err != nil {
		return err
	}

	s.logger.Info().Msgf("PawaPay deposit request: deposit_id=%s, amount=%f, currency=%s, phone_number=%s, provider=%s", depositID.String(), amountValue.Float64, currency, phoneNumber, pawapayProvider)
	
	// Trims the '+' only if it is at the beginning
	cleanNumber := strings.TrimPrefix(phoneNumber, "+")
	
	fmt.Println(cleanNumber) // Output: 237654131027

	s.logger.Info().Msg("Constructing request to PawaPay InitiateDeposit API...")
	req := &pawapaydeposits.InitiateDepositRequest{
		DepositID: depositID.String(),
		Amount:    strconv.FormatFloat(amountValue.Float64, 'f', 2, 64),
		Currency:  currency,
		Payer: pawapaydeposits.Payer{
			Type: "MMO",
			AccountDetails: pawapaydeposits.AccountDetails{
				PhoneNumber: cleanNumber,
				Provider:    pawapayProvider,
			},
		},
	}

	s.logger.Info().Msg("Sending request to PawaPay InitiateDeposit API...")
	_, err = s.pawapayClient.Deposits.InitiateDeposit(ctx, req)

	s.logger.Info().Msg("No errors logged from PawaPay InitiateDeposit API...")
	return err
}

// GetDepositByGHLTransactionID fetches a deposit by its GoHighLevel
// transaction identifier. This is used by the GHL Custom Payment Provider
// query endpoint to correlate a HighLevel transaction with an RVPay deposit.
func (s *Impl) GetDepositByGHLTransactionID(ctx context.Context, req *transactionsgrpc.GetDepositByGHLTransactionIDRequest) (*transactionsgrpc.GetDepositByGHLTransactionIDResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "deposit request is required")
	}

	ghlTransactionID := strings.TrimSpace(req.GetGhlTransactionId())
	if ghlTransactionID == "" {
		return nil, status.Error(codes.InvalidArgument, "ghl_transaction_id is required")
	}

	deposit, err := s.depositRepo.GetByGHLTransactionID(ctx, ghlTransactionID)
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrNotFound):
			return nil, status.Error(codes.NotFound, "deposit not found")
		default:
			s.logger.Error().Err(err).Str("ghl_transaction_id", ghlTransactionID).Msg("could not get deposit by GHL transaction id")
			return nil, status.Error(codes.Internal, "could not get deposit")
		}
	}

	return &transactionsgrpc.GetDepositByGHLTransactionIDResponse{
		Deposit: depositToProto(deposit),
	}, nil
}

// GetDeposit fetches a deposit by id.
func (s *Impl) GetDeposit(ctx context.Context, req *transactionsgrpc.GetDepositRequest) (*transactionsgrpc.GetDepositResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "deposit request is required")
	}

	depositID, err := uuid.Parse(req.GetDepositId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "deposit_id must be a valid UUID")
	}

	deposit, err := s.depositRepo.GetByID(ctx, depositID)
	if err != nil {
		switch {
		case errors.Is(err, repo.ErrNotFound):
			return nil, status.Error(codes.NotFound, "deposit not found")
		default:
			s.logger.Error().Err(err).Str("deposit_id", depositID.String()).Msg("could not get deposit")
			return nil, status.Error(codes.Internal, "could not get deposit")
		}
	}

	return &transactionsgrpc.GetDepositResponse{
		Deposit: depositToProto(deposit),
	}, nil
}

// validateAmount validates and converts a protobuf Money amount to pgtype.Numeric.
func validateAmount(money *commongrpc.Money) (pgtype.Numeric, error) {
	if money == nil {
		return pgtype.Numeric{}, status.Error(codes.InvalidArgument, "amount is required")
	}

	var amount pgtype.Numeric
	if err := amount.Scan(money.GetAmount()); err != nil {
		return pgtype.Numeric{}, status.Errorf(codes.InvalidArgument, "invalid deposit amount: %v", err)
	}

	f, err := amount.Float64Value()
	if err != nil {
		return pgtype.Numeric{}, status.Errorf(codes.InvalidArgument, "invalid deposit amount: %v", err)
	}
	if !f.Valid || f.Float64 <= 0 {
		return pgtype.Numeric{}, status.Error(codes.InvalidArgument, "deposit amount must be greater than zero")
	}

	return amount, nil
}

func grpcPaymentTypeToSqlc(paymentType commongrpc.PaymentType) (sqlc.PaymentType, error) {
	switch paymentType {
	case commongrpc.PaymentType_PAYMENT_TYPE_MMO:
		return sqlc.PaymentTypeMMO, nil
	case commongrpc.PaymentType_PAYMENT_TYPE_CREDIT_CARD:
		return sqlc.PaymentTypeCREDITCARD, nil
	default:
		return "", status.Errorf(codes.InvalidArgument, "unsupported payment type: %s", paymentType)
	}
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
