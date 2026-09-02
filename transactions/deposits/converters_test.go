package deposits

import (
	"testing"

	transactionsgrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/transactionsgrpc"
	sqlc "github.com/I-Frostbyte/rvpay-go/transactions/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

// newTestDeposit returns a deposit row shaped like a freshly initiated one:
// every nullable text column is SQL NULL (nil in Go) and the optional
// lifecycle timestamps are NULL.
func newTestDeposit() sqlc.Deposit {
	return sqlc.Deposit{
		ID:                uuid.New(),
		ClientName:        "highlevel-ZVqmceTBDIe1a6Hscxeo",
		CustomerID:        nil,
		MerchantID:        nil,
		Amount:            pgtype.Numeric{},
		Currency:          "XAF",
		PaymentType:       sqlc.PaymentTypeMMO,
		PayerPhoneNumber:  "+237600000000",
		Provider:          sqlc.PaymentProviderMTNMOMO,
		Status:            sqlc.DepositStatusINITIATED,
		ExternalReference: nil,
		CompletedAt:       pgtype.Timestamptz{},
		FailedAt:          pgtype.Timestamptz{},
		FailureReason:     nil,
		GhlTransactionID:  nil,
		GhlChargeID:       nil,
	}
}

// TestDepositToProtoCase1NewlyInitiated verifies Case 1: a newly initiated
// deposit whose nullable fields are all SQL NULL scans through persistence
// and converts to the wire contract without panic or fabricated values.
func TestDepositToProtoCase1NewlyInitiated(t *testing.T) {
	t.Parallel()

	proto := depositToProto(newTestDeposit())

	if proto.Id == "" || proto.ClientName != "highlevel-ZVqmceTBDIe1a6Hscxeo" {
		t.Fatalf("unexpected proto deposit: %+v", proto)
	}
	if proto.CustomerId != "" || proto.MerchantId != "" {
		t.Fatalf("NULL customer_id/merchant_id must surface as empty strings, got %q/%q", proto.CustomerId, proto.MerchantId)
	}
	if proto.ExternalReference != "" || proto.FailureReason != "" {
		t.Fatalf("NULL external_reference/failure_reason must surface as empty strings, got %q/%q", proto.ExternalReference, proto.FailureReason)
	}
	if proto.CompletedAt != nil || proto.FailedAt != nil {
		t.Fatalf("NULL completed_at/failed_at must not be set, got %v/%v", proto.CompletedAt, proto.FailedAt)
	}
}

// TestDepositToProtoCase2ExternalIdentifiersOnly verifies Case 2: the external
// HighLevel identifiers may be present while the legacy nullable fields remain
// NULL, and the exact external values propagate unchanged.
func TestDepositToProtoCase2ExternalIdentifiersOnly(t *testing.T) {
	t.Parallel()

	customerID := "ghl-contact-123"
	merchantID := "ghl-transaction-456"

	deposit := newTestDeposit()
	deposit.CustomerID = &customerID
	deposit.MerchantID = &merchantID

	proto := depositToProto(deposit)

	if proto.CustomerId != "ghl-contact-123" {
		t.Fatalf("customer_id = %q, want exact external value", proto.CustomerId)
	}
	if proto.MerchantId != "ghl-transaction-456" {
		t.Fatalf("merchant_id = %q, want exact external value", proto.MerchantId)
	}
}

// TestDepositToProtoCase3PopulatedNullableValues verifies Case 3: nullable
// fields containing real values keep their exact values through conversion.
func TestDepositToProtoCase3PopulatedNullableValues(t *testing.T) {
	t.Parallel()

	externalRef := "some-provider-reference"
	ghlTx := "GHL-TX-1"
	ghlCharge := "GHL-CHARGE-1"
	failure := "provider rejected"
	completedAt := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}

	deposit := newTestDeposit()
	deposit.Status = sqlc.DepositStatusCOMPLETED
	deposit.ExternalReference = &externalRef
	deposit.GhlTransactionID = &ghlTx
	deposit.GhlChargeID = &ghlCharge
	deposit.FailureReason = &failure
	deposit.CompletedAt = completedAt

	proto := depositToProto(deposit)

	if proto.ExternalReference != "some-provider-reference" {
		t.Fatalf("external_reference = %q, want exact provider reference", proto.ExternalReference)
	}
	if proto.CompletedAt == nil || proto.CompletedAt.AsTime() != completedAt.Time {
		t.Fatalf("completed_at not propagated: %v", proto.CompletedAt)
	}
	if proto.Status != transactionsgrpc.DepositStatus_DEPOSIT_STATUS_COMPLETED {
		t.Fatalf("status = %v, want COMPLETED", proto.Status)
	}
	if proto.Amount == nil || proto.Amount.Currency != "XAF" {
		t.Fatalf("payment fields must remain intact: %+v", proto.Amount)
	}
}
