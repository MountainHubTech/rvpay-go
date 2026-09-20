package ghldeliver

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/MountainHubTech/rvpay-go/transactions/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// testDeposit builds a finalized COMPLETED deposit fixture with the
// authoritative persisted fields a real PawaPay-confirmed payment carries.
func testDeposit(clientName string) sqlc.Deposit {
	var amount pgtype.Numeric
	if err := amount.Scan("2.00"); err != nil {
		panic(err)
	}
	idempotencyKey := uuid.New()
	contactID := "TL2jQXT39zCqP8dVZsKx"
	orderID := "6aa60c9565a70758a0d4fbc8"
	transactionID := "test-transaction-001"
	providerTransactionID := "test-pawapay-001"
	return sqlc.Deposit{
		ID:                uuid.New(),
		ClientName:        clientName,
		CustomerID:        &contactID,
		Amount:            amount,
		Currency:          "XAF",
		PayerPhoneNumber:  "654131027",
		Status:            sqlc.DepositStatusCOMPLETED,
		ExternalReference: &providerTransactionID,
		IdempotencyKey:    idempotencyKey,
		CompletedAt:       pgtype.Timestamptz{Time: time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC), Valid: true},
		GhlTransactionID:  &transactionID,
		GhlOrderID:        &orderID,
	}
}

func testNameCustomer(name *string) *sqlc.Customer {
	return &sqlc.Customer{ClientName: "highlevel-loc123", PhoneNumber: "654131027", Name: name}
}

// TestBuildPaymentCompletedEventContract verifies the exact agreed field
// names and the authoritative persisted values.
func TestBuildPaymentCompletedEventContract(t *testing.T) {
	t.Parallel()

	deposit := testDeposit("highlevel-rVBsqXAKXlYLz0C96kA8")
	name := "Gilbert Test"
	eventID := uuid.New().String()

	_, payload, err := BuildPaymentCompletedEvent(deposit, testNameCustomer(&name), eventID)
	if err != nil {
		t.Fatalf("BuildPaymentCompletedEvent failed: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}

	// The exact agreed contract: field names must never be renamed or
	// re-cased, and no additional fields may appear.
	want := []string{
		"event", "eventId", "paymentId", "depositId", "orderId", "transactionId",
		"locationId", "contactId", "customerEmail", "customerPhone", "firstName",
		"lastName", "amount", "currency", "status", "provider",
		"providerTransactionId", "productName", "idempotencyKey", "occurredAt",
	}
	if len(raw) != len(want) {
		t.Fatalf("payload has %d fields, want %d: %v", len(raw), len(want), raw)
	}
	for _, field := range want {
		if _, ok := raw[field]; !ok {
			t.Fatalf("payload missing contract field %q", field)
		}
	}

	var decoded PaymentCompletedEvent
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode event: %v", err)
	}

	if decoded.Event != "rvpay.payment.completed" {
		t.Errorf("event = %q, want rvpay.payment.completed", decoded.Event)
	}
	if decoded.Status != "paid" {
		t.Errorf("status = %q, want paid", decoded.Status)
	}
	if decoded.Provider != "pawapay" {
		t.Errorf("provider = %q, want pawapay", decoded.Provider)
	}
	if decoded.EventID != eventID {
		t.Errorf("eventId = %q, want the stable outbox event id", decoded.EventID)
	}
	// paymentId and idempotencyKey are the same unique payment identity.
	if decoded.PaymentID == "" || decoded.PaymentID != decoded.IdempotencyKey {
		t.Errorf("paymentId/idempotencyKey mismatch: %q vs %q", decoded.PaymentID, decoded.IdempotencyKey)
	}
	if decoded.DepositID != deposit.ID.String() {
		t.Errorf("depositId = %q, want %q", decoded.DepositID, deposit.ID)
	}
	// The HighLevel locationId comes from the authoritative persisted
	// client_name convention, never from a browser value.
	if decoded.LocationID != "rVBsqXAKXlYLz0C96kA8" {
		t.Errorf("locationId = %q, want rVBsqXAKXlYLz0C96kA8", decoded.LocationID)
	}
	if decoded.ContactID != "TL2jQXT39zCqP8dVZsKx" {
		t.Errorf("contactId = %q, want the persisted HighLevel contact id", decoded.ContactID)
	}
	if decoded.OrderID != "6aa60c9565a70758a0d4fbc8" {
		t.Errorf("orderId = %q, want the persisted ghl_order_id", decoded.OrderID)
	}
	if decoded.TransactionID != "test-transaction-001" {
		t.Errorf("transactionId = %q, want the persisted ghl_transaction_id", decoded.TransactionID)
	}
	if decoded.ProviderTransactionID != "test-pawapay-001" {
		t.Errorf("providerTransactionId = %q, want the persisted PawaPay reference", decoded.ProviderTransactionID)
	}
	if decoded.CustomerPhone != "654131027" {
		t.Errorf("customerPhone = %q, want the persisted payer phone number", decoded.CustomerPhone)
	}
	if decoded.FirstName != "Gilbert" || decoded.LastName != "Test" {
		t.Errorf("names = %q/%q, want Gilbert/Test from the persisted customer name", decoded.FirstName, decoded.LastName)
	}
	if decoded.Currency != "XAF" {
		t.Errorf("currency = %q, want XAF", decoded.Currency)
	}
	if decoded.Amount != json.Number("2") {
		t.Errorf("amount = %q, want the exact persisted amount 2 (zero-decimal rendering of 2.00)", decoded.Amount)
	}
	if decoded.OccurredAt != "2026-09-18T12:00:00Z" {
		t.Errorf("occurredAt = %q, want the authoritative completed_at", decoded.OccurredAt)
	}
	// Documented limitation: RVPay does not persist a customer email or a
	// product name; they are never substituted with unrelated data.
	if decoded.CustomerEmail != "" {
		t.Errorf("customerEmail = %q, want empty (not persisted; never substituted)", decoded.CustomerEmail)
	}
	if decoded.ProductName != "" {
		t.Errorf("productName = %q, want empty (not persisted; never substituted)", decoded.ProductName)
	}
}

// TestBuildPaymentCompletedEventLocationGuards verifies a location is never
// fabricated from a non-conforming persisted client_name.
func TestBuildPaymentCompletedEventLocationGuards(t *testing.T) {
	t.Parallel()

	for _, clientName := range []string{"", "highlevel-", "lowlevel-abc", "some-other-client"} {
		_, payload, err := BuildPaymentCompletedEvent(testDeposit(clientName), nil, uuid.New().String())
		if err != nil {
			t.Fatalf("BuildPaymentCompletedEvent(%q) failed: %v", clientName, err)
		}
		var decoded PaymentCompletedEvent
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if decoded.LocationID != "" {
			t.Errorf("clientName %q: locationId = %q, want empty for a non-conforming name", clientName, decoded.LocationID)
		}
	}
}

// TestBuildPaymentCompletedEventInvalidAmount verifies a corrupt amount is
// refused rather than fabricated.
func TestBuildPaymentCompletedEventInvalidAmount(t *testing.T) {
	t.Parallel()

	deposit := testDeposit("highlevel-loc123")
	deposit.Amount = pgtype.Numeric{Valid: false}
	if _, _, err := BuildPaymentCompletedEvent(deposit, nil, uuid.New().String()); err == nil {
		t.Fatal("expected an error for an invalid persisted amount, got nil")
	}
}

// TestValidateInboundWebhookURL verifies the HTTPS-only configuration gate.
func TestValidateInboundWebhookURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		url  string
		want bool
	}{
		{"", false},
		{"http://example.com/hook", false},
		{"ftp://example.com/hook", false},
		{"https://", false},
		{"example.com/hook", false},
		{"https://example.com/workflow/hook", true},
		{"https://hooks.example.com:8443/x?y=1", true},
	}
	for _, tc := range cases {
		if got := ValidateInboundWebhookURL(tc.url); got != tc.want {
			t.Errorf("ValidateInboundWebhookURL(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}
