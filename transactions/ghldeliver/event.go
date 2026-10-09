// Package ghldeliver implements the durable outbound delivery of the
// "rvpay.payment.completed" event to the HighLevel Inbound Webhook workflow.
//
// The event is persisted in the payment_events outbox (in the same database
// transaction as the authoritative PawaPay COMPLETED deposit transition) and
// delivered asynchronously by the worker in this package. HighLevel
// availability never determines whether a RVPay payment is successful.
//
// The HighLevel inbound webhook URL is environment-level secret
// configuration (HIGHLEVEL_INBOUND_WEBHOOK_URL); it is never stored in the
// database, never hard-coded, and never logged.
package ghldeliver

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/MountainHubTech/rvpay-go/transactions/db/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// EventType is the exact agreed event name consumed by the HighLevel
// workflow's If/Else branch.
const EventType = "rvpay.payment.completed"

// EventStatusPaid is the exact status value for confirmed successful payments.
const EventStatusPaid = "paid"

// EventProvider is the exact provider value for PawaPay-collected payments.
const EventProvider = "pawapay"

// locationClientNamePrefix is the RVPay client-name convention binding a
// deposit to its HighLevel location ("highlevel-<locationId>"). It matches
// the derivation already used by the ghlsync worker.
const locationClientNamePrefix = "highlevel-"

// PaymentCompletedEvent is the exact agreed outbound JSON contract. Field
// names are the contract and must never be renamed or re-cased.
type PaymentCompletedEvent struct {
	Event                 string      `json:"event"`
	EventID               string      `json:"eventId"`
	PaymentID             string      `json:"paymentId"`
	DepositID             string      `json:"depositId"`
	OrderID               string      `json:"orderId"`
	TransactionID         string      `json:"transactionId"`
	LocationID            string      `json:"locationId"`
	ContactID             string      `json:"contactId"`
	CustomerEmail         string      `json:"customerEmail"`
	CustomerPhone         string      `json:"customerPhone"`
	FirstName             string      `json:"firstName"`
	LastName              string      `json:"lastName"`
	Amount                json.Number `json:"amount"`
	Currency              string      `json:"currency"`
	Status                string      `json:"status"`
	Provider              string      `json:"provider"`
	ProviderTransactionID string      `json:"providerTransactionId"`
	ProductName           string      `json:"productName"`
	IdempotencyKey        string      `json:"idempotencyKey"`
	OccurredAt            string      `json:"occurredAt"`
}

// BuildPaymentCompletedEvent builds the event payload exclusively from
// authoritative persisted RVPay data: the finalized deposit row (in its
// terminal COMPLETED state), the stable outbox event id, and the persisted
// customer row when one exists.
//
// Field sourcing:
//   - eventId         payment_events.event_id — stable across all retries.
//   - locationId      deposits.client_name ("highlevel-<locationId>"
//     convention); non-conforming names yield "" — a location is never
//     fabricated and never taken from an untrusted request.
//   - contactId       deposits.customer_id (the HighLevel contact id).
//   - orderId         deposits.ghl_order_id.
//   - transactionId   deposits.ghl_transaction_id.
//   - paymentId / idempotencyKey   the deposit idempotency key — the unique
//     payment identity persisted at initiation (matches the agreed example
//     payload where both fields carry the same payment identity).
//   - customerPhone   deposits.payer_phone_number.
//   - firstName/lastName  customers.name (the persisted display name), split
//     on the first space; a single token populates firstName only.
//   - customerEmail   customers.email when persisted, else "". RVPay does not
//     currently persist a customer email anywhere (documented limitation).
//   - providerTransactionId   deposits.external_reference (the PawaPay V2
//     transaction id preserved at callback time); "" when PawaPay did not
//     supply one.
//   - productName     "" — not persisted anywhere in the current RVPay data
//     model (documented limitation; the HighLevel workflow does not consume
//     it).
//   - occurredAt      deposits.completed_at (authoritative completion time),
//     RFC 3339 UTC.
func BuildPaymentCompletedEvent(deposit sqlc.Deposit, customer *sqlc.Customer, eventID string) (PaymentCompletedEvent, []byte, error) {
	event := PaymentCompletedEvent{
		Event:                 EventType,
		EventID:               eventID,
		PaymentID:             deposit.IdempotencyKey.String(),
		DepositID:             deposit.ID.String(),
		OrderID:               textValue(deposit.GhlOrderID),
		TransactionID:         textValue(deposit.GhlTransactionID),
		LocationID:            locationIDFromClientName(deposit.ClientName),
		ContactID:             textValue(deposit.CustomerID),
		CustomerPhone:         deposit.PayerPhoneNumber,
		Amount:                json.Number("0"),
		Currency:              deposit.Currency,
		Status:                EventStatusPaid,
		Provider:              EventProvider,
		ProviderTransactionID: textValue(deposit.ExternalReference),
		ProductName:           "",
		IdempotencyKey:        deposit.IdempotencyKey.String(),
		OccurredAt:            occurredAt(deposit.CompletedAt),
	}

	if customer != nil {
		event.FirstName, event.LastName = splitPersonName(textValue(customer.Name))
		// customerEmail stays "": RVPay does not persist a customer email
		// anywhere (customers carries name/address/phone only). This is a
		// documented data-model limitation, not a substitution; the smallest
		// future change is persisting the checkout-supplied customer email on
		// the customers row and mapping it here.
	}

	amount, err := numericAmountJSON(deposit.Amount)
	if err != nil {
		return PaymentCompletedEvent{}, nil, fmt.Errorf("serialize deposit amount: %w", err)
	}
	event.Amount = amount

	payload, err := json.Marshal(event)
	if err != nil {
		return PaymentCompletedEvent{}, nil, fmt.Errorf("encode payment completed event: %w", err)
	}
	return event, payload, nil
}

// occurredAt renders the authoritative completion timestamp in RFC 3339 UTC.
// A terminal COMPLETED deposit always has completed_at set; the guard only
// protects against an anomalous row.
func occurredAt(completedAt pgtype.Timestamptz) string {
	if !completedAt.Valid {
		return time.Now().UTC().Format(time.RFC3339)
	}
	return completedAt.Time.UTC().Format(time.RFC3339)
}

// textValue dereferences a nullable-text column, mapping SQL NULL to "".
func textValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// locationIDFromClientName derives the HighLevel locationId from the RVPay
// client-name convention "highlevel-<locationId>". It returns "" for any
// non-conforming name so the event is never built against a fabricated
// location.
func locationIDFromClientName(clientName string) string {
	if len(clientName) <= len(locationClientNamePrefix) {
		return ""
	}
	if clientName[:len(locationClientNamePrefix)] != locationClientNamePrefix {
		return ""
	}
	return clientName[len(locationClientNamePrefix):]
}

// splitPersonName splits the persisted display name on the first space into
// the HighLevel firstName/lastName fields. A single token populates only
// firstName; an empty name yields both fields empty.
func splitPersonName(name string) (string, string) {
	for i := 0; i < len(name); i++ {
		if name[i] == ' ' {
			return name[:i], name[i+1:]
		}
	}
	if name == "" {
		return "", ""
	}
	return name, ""
}

// numericAmountJSON serializes a NUMERIC(18,2) deposit amount as an exact
// JSON number without any floating-point conversion: the mantissa is shifted
// by the numeric exponent to produce the exact decimal string, so a confirmed
// payment amount is never rounded or truncated. Any amount PawaPay accepted
// is representable this way; the error path exists only to refuse building an
// event from a corrupt amount rather than fabricating a value.
func numericAmountJSON(amount pgtype.Numeric) (json.Number, error) {
	if !amount.Valid || amount.NaN || amount.Int == nil {
		return "", fmt.Errorf("amount is not a valid number")
	}

	digits := new(big.Int).Abs(amount.Int).String()
	if amount.Exp >= 0 {
		digits += strings.Repeat("0", int(amount.Exp))
	} else {
		k := int(-amount.Exp)
		if len(digits) <= k {
			digits = strings.Repeat("0", k-len(digits)+1) + digits
		}
		frac := digits[len(digits)-k:]
		whole := digits[:len(digits)-k]
		frac = strings.TrimRight(frac, "0")
		if frac != "" {
			digits = whole + "." + frac
		} else {
			digits = whole
		}
	}
	if amount.Int.Sign() < 0 {
		digits = "-" + digits
	}
	return json.Number(digits), nil
}
