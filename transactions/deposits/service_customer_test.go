package deposits

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/I-Frostbyte/pawapay_client"
	transactionsgrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/transactionsgrpc"
	"github.com/I-Frostbyte/rvpay-go/transactions/db/repo"
	repomocks "github.com/I-Frostbyte/rvpay-go/transactions/db/repo/mocks"
	"github.com/I-Frostbyte/rvpay-go/transactions/db/sqlc"
	sqlcmocks "github.com/I-Frostbyte/rvpay-go/transactions/db/sqlc/mocks"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// customerRequest returns a valid deposit request carrying customer
// information, as the Admin Dashboard payment page now sends it.
func customerRequest() *transactionsgrpc.CreateDepositRequest {
	req := validCreateRequest()
	req.Customer = &transactionsgrpc.Customer{
		ClientName:  "highlevel-abc123",
		Name:        "Achah Rosine",
		PhoneNumber: "+237600000000",
		Address:     "Buea Cameroon",
	}
	return req
}

// expectedCustomerCreate mocks the customer resolve-or-create step at the
// tx-scoped sqlc.Querier level (the service wraps txQuerier in a customer
// repo) and asserts the persisted customer fields (directive Tests 8 and 9).
func expectedCustomerCreate(t *testing.T, querier *sqlcmocks.MockQuerier, clientName, name, address string) {
	t.Helper()
	querier.EXPECT().GetCustomerByClientNameAndPhone(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, p sqlc.GetCustomerByClientNameAndPhoneParams) (sqlc.Customer, error) {
			if p.ClientName != clientName || p.PhoneNumber != "+237600000000" {
				t.Fatalf("lookup args = %+v, want client_name=%q phone=%q", p, clientName, "+237600000000")
			}
			return sqlc.Customer{}, repo.ErrNotFound
		})
	querier.EXPECT().CreateCustomer(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, p sqlc.CreateCustomerParams) (sqlc.Customer, error) {
			if p.ClientName != clientName || p.PhoneNumber != "+237600000000" {
				t.Fatalf("create args = %+v, want client_name=%q phone=%q", p, clientName, "+237600000000")
			}
			if p.MerchantID.Valid {
				t.Fatalf("merchant_id must be NULL for external payment flow, got %+v", p.MerchantID)
			}
			if p.Name == nil || *p.Name != name {
				t.Fatalf("customer name = %v, want %q", p.Name, name)
			}
			if p.Address == nil || *p.Address != address {
				t.Fatalf("customer address = %v, want %q", p.Address, address)
			}
			return sqlc.Customer{ID: uuid.New(), ClientName: p.ClientName, PhoneNumber: p.PhoneNumber, Name: p.Name, Address: p.Address}, nil
		})
}

// pawapayServer returns an httptest server responding with the given PawaPay
// initiation payload; the real PawaPay API is never contacted.
func pawapayServer(t *testing.T, body string, code int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(body))
	}))
}

// Test 1 (directive §18): Customer + Deposit + PawaPay ACCEPTED — the
// customer is created, the deposit is created, and the transaction commits.
func TestCustomerDepositPawapayAcceptedCommits(t *testing.T) {
	t.Parallel()

	srv := pawapayServer(t, `{"depositId":"dep-1","status":"ACCEPTED"}`, http.StatusOK)
	defer srv.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tx := &fakeTx{}
	querier, txRepo := beginTx(ctrl, tx)
	expectedCustomerCreate(t, querier, "highlevel-abc123", "Achah Rosine", "Buea Cameroon")
	querier.EXPECT().CreateDeposit(gomock.Any(), gomock.Any()).Return(sqlc.Deposit{ID: uuid.New()}, nil)

	service := newTestServiceWithCustomers(repomocks.NewMockDepositRepo(ctrl), txRepo, nil, *pawapay_client.NewClient(srv.URL, "test-key"))

	if _, err := service.InitiateDeposit(context.Background(), customerRequest()); err != nil {
		t.Fatalf("InitiateDeposit failed: %v", err)
	}
	if !tx.committed || tx.rolledBack {
		t.Fatal("transaction should have been committed, not rolled back")
	}
}

// Test 3 (directive §18): PawaPay REJECTED — the transaction rolls back, so
// neither the customer nor the deposit is committed.
func TestCustomerNotCommittedOnPawapayRejected(t *testing.T) {
	t.Parallel()

	srv := pawapayServer(t, `{"depositId":"dep-1","status":"REJECTED","failureReason":{"failureCode":"INVALID_AMOUNT","failureMessage":"invalid"}}`, http.StatusOK)
	defer srv.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tx := &fakeTx{}
	querier, txRepo := beginTx(ctrl, tx)
	expectedCustomerCreate(t, querier, "highlevel-abc123", "Achah Rosine", "Buea Cameroon")
	querier.EXPECT().CreateDeposit(gomock.Any(), gomock.Any()).Return(sqlc.Deposit{ID: uuid.New()}, nil)

	service := newTestServiceWithCustomers(repomocks.NewMockDepositRepo(ctrl), txRepo, nil, *pawapay_client.NewClient(srv.URL, "test-key"))

	_, err := service.InitiateDeposit(context.Background(), customerRequest())
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("status code = %s, want %s", got, codes.Internal)
	}
	if !tx.rolledBack || tx.committed {
		t.Fatal("REJECTED must roll back: customer and deposit must not be committed")
	}
}

// Test 4 (directive §18): PawaPay unexpected status — rollback, no customer
// and no deposit committed.
func TestCustomerNotCommittedOnUnexpectedPawapayStatus(t *testing.T) {
	t.Parallel()

	srv := pawapayServer(t, `{"depositId":"dep-1","status":"SOMETHING_ELSE"}`, http.StatusOK)
	defer srv.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tx := &fakeTx{}
	querier, txRepo := beginTx(ctrl, tx)
	expectedCustomerCreate(t, querier, "highlevel-abc123", "Achah Rosine", "Buea Cameroon")
	querier.EXPECT().CreateDeposit(gomock.Any(), gomock.Any()).Return(sqlc.Deposit{ID: uuid.New()}, nil)

	service := newTestServiceWithCustomers(repomocks.NewMockDepositRepo(ctrl), txRepo, nil, *pawapay_client.NewClient(srv.URL, "test-key"))

	_, err := service.InitiateDeposit(context.Background(), customerRequest())
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("status code = %s, want %s", got, codes.Internal)
	}
	if !tx.rolledBack || tx.committed {
		t.Fatal("unexpected status must roll back: customer and deposit must not be committed")
	}
}

// Test 5 (directive §18): customer creation failure — the deposit INSERT is
// never attempted and nothing is committed.
func TestDepositNotCommittedOnCustomerFailure(t *testing.T) {
	t.Parallel()

	srv := pawapayServer(t, `{"depositId":"dep-1","status":"ACCEPTED"}`, http.StatusOK)
	defer srv.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tx := &fakeTx{}
	querier, txRepo := beginTx(ctrl, tx)
	querier.EXPECT().GetCustomerByClientNameAndPhone(gomock.Any(), gomock.Any()).
		Return(sqlc.Customer{}, repo.ErrNotFound)
	querier.EXPECT().CreateCustomer(gomock.Any(), gomock.Any()).
		Return(sqlc.Customer{}, context.DeadlineExceeded)
	// CreateDeposit is deliberately NOT expected: gomock fails the test if
	// the deposit INSERT is attempted after the customer failure.

	service := newTestServiceWithCustomers(repomocks.NewMockDepositRepo(ctrl), txRepo, nil, *pawapay_client.NewClient(srv.URL, "test-key"))

	_, err := service.InitiateDeposit(context.Background(), customerRequest())
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("status code = %s, want %s", got, codes.Internal)
	}
	if !tx.rolledBack || tx.committed {
		t.Fatal("customer failure must roll back the transaction")
	}
}

// Test 6 (directive §18): deposit creation failure — the customer is not
// committed.
func TestCustomerNotCommittedOnDepositFailure(t *testing.T) {
	t.Parallel()

	srv := pawapayServer(t, `{"depositId":"dep-1","status":"ACCEPTED"}`, http.StatusOK)
	defer srv.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tx := &fakeTx{}
	querier, txRepo := beginTx(ctrl, tx)
	expectedCustomerCreate(t, querier, "highlevel-abc123", "Achah Rosine", "Buea Cameroon")
	querier.EXPECT().CreateDeposit(gomock.Any(), gomock.Any()).Return(sqlc.Deposit{}, context.DeadlineExceeded)

	service := newTestServiceWithCustomers(repomocks.NewMockDepositRepo(ctrl), txRepo, nil, *pawapay_client.NewClient(srv.URL, "test-key"))

	_, err := service.InitiateDeposit(context.Background(), customerRequest())
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("status code = %s, want %s", got, codes.Internal)
	}
	if !tx.rolledBack || tx.committed {
		t.Fatal("deposit failure must roll back the transaction: customer must not be committed")
	}
}

// Test 7 (directive §18): commit failure — Internal error; the deferred
// rollback runs (nothing is committed).
func TestCommitFailureReturnsInternal(t *testing.T) {
	t.Parallel()

	srv := pawapayServer(t, `{"depositId":"dep-1","status":"ACCEPTED"}`, http.StatusOK)
	defer srv.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tx := &fakeTx{commitErr: context.DeadlineExceeded}
	querier, txRepo := beginTx(ctrl, tx)
	expectedCustomerCreate(t, querier, "highlevel-abc123", "Achah Rosine", "Buea Cameroon")
	querier.EXPECT().CreateDeposit(gomock.Any(), gomock.Any()).Return(sqlc.Deposit{ID: uuid.New()}, nil)

	service := newTestServiceWithCustomers(repomocks.NewMockDepositRepo(ctrl), txRepo, nil, *pawapay_client.NewClient(srv.URL, "test-key"))

	_, err := service.InitiateDeposit(context.Background(), customerRequest())
	if got := status.Code(err); got != codes.Internal {
		t.Fatalf("status code = %s, want %s", got, codes.Internal)
	}
	if tx.committed {
		t.Fatal("failed commit must not be reported as committed")
	}
}

// Test 9 (directive §18): the customer is associated with the external
// client_name "highlevel-<locationId>".
func TestCustomerAssociatedWithClientName(t *testing.T) {
	t.Parallel()

	srv := pawapayServer(t, `{"depositId":"dep-1","status":"ACCEPTED"}`, http.StatusOK)
	defer srv.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var gotClientName string
	tx := &fakeTx{}
	querier, txRepo := beginTx(ctrl, tx)
	querier.EXPECT().GetCustomerByClientNameAndPhone(gomock.Any(), gomock.Any()).
		Return(sqlc.Customer{}, repo.ErrNotFound)
	querier.EXPECT().CreateCustomer(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, p sqlc.CreateCustomerParams) (sqlc.Customer, error) {
			gotClientName = p.ClientName
			return sqlc.Customer{ID: uuid.New(), ClientName: p.ClientName}, nil
		})
	querier.EXPECT().CreateDeposit(gomock.Any(), gomock.Any()).Return(sqlc.Deposit{ID: uuid.New()}, nil)

	service := newTestServiceWithCustomers(repomocks.NewMockDepositRepo(ctrl), txRepo, nil, *pawapay_client.NewClient(srv.URL, "test-key"))

	if _, err := service.InitiateDeposit(context.Background(), customerRequest()); err != nil {
		t.Fatalf("InitiateDeposit failed: %v", err)
	}
	if gotClientName != "highlevel-abc123" {
		t.Fatalf("customer client_name = %q, want %q", gotClientName, "highlevel-abc123")
	}
}

// Existing-customer reuse: no duplicate customer rows are created for repeat
// payments from the same client/phone (directive §16).
func TestExistingCustomerReused(t *testing.T) {
	t.Parallel()

	srv := pawapayServer(t, `{"depositId":"dep-1","status":"ACCEPTED"}`, http.StatusOK)
	defer srv.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tx := &fakeTx{}
	querier, txRepo := beginTx(ctrl, tx)
	querier.EXPECT().GetCustomerByClientNameAndPhone(gomock.Any(), gomock.Any()).
		Return(sqlc.Customer{ID: uuid.New(), ClientName: "highlevel-abc123", PhoneNumber: "+237600000000"}, nil)
	// CreateCustomer is deliberately NOT expected: an existing customer
	// must be reused, never re-inserted.
	querier.EXPECT().CreateDeposit(gomock.Any(), gomock.Any()).Return(sqlc.Deposit{ID: uuid.New()}, nil)

	service := newTestServiceWithCustomers(repomocks.NewMockDepositRepo(ctrl), txRepo, nil, *pawapay_client.NewClient(srv.URL, "test-key"))

	if _, err := service.InitiateDeposit(context.Background(), customerRequest()); err != nil {
		t.Fatalf("InitiateDeposit failed: %v", err)
	}
	if !tx.committed || tx.rolledBack {
		t.Fatal("transaction should have been committed")
	}
}
