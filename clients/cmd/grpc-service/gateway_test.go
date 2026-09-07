package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	clientsgrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/clientsgrpc"
	commongrpc "github.com/I-Frostbyte/rvpay-go/grpc/go/commongrpc"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// fakeClientsService implements clientsgrpc.ClientsServiceServer for gateway
// wiring tests. It embeds the generated Unimplemented type so any RPC not
// overridden fails with codes.Unimplemented, matching forward-compatible
// service behaviour.
type fakeClientsService struct {
	clientsgrpc.UnimplementedClientsServiceServer

	getClientErr error
}

func (f *fakeClientsService) GetClient(_ context.Context, req *clientsgrpc.GetClientRequest) (*clientsgrpc.GetClientResponse, error) {
	if f.getClientErr != nil {
		return nil, f.getClientErr
	}
	return &clientsgrpc.GetClientResponse{
		Client: &clientsgrpc.Client{
			Id:        req.GetId(),
			Name:      "Acme Corp",
			Status:    commongrpc.ClientStatus_CLIENT_STATUS_ACTIVE,
			CreatedAt: timestamppb.New(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)),
		},
	}, nil
}

func (f *fakeClientsService) ListSubAccounts(_ context.Context, req *clientsgrpc.ListSubAccountsRequest) (*clientsgrpc.ListSubAccountsResponse, error) {
	return &clientsgrpc.ListSubAccountsResponse{
		Rows: []*clientsgrpc.SubAccountRow{
			{Id: "sub-1", Name: "Acme Store", Location: "loc_1", Initials: "AS", Status: clientsgrpc.SubAccountStatus_SUB_ACCOUNT_STATUS_ACTIVE},
		},
		Total:    1,
		Page:     req.GetPage(),
		PageSize: req.GetPageSize(),
	}, nil
}

// newClientsGateway constructs the exact gateway wiring used by
// clients/cmd/grpc-service/main.go: a grpc-gateway runtime.ServeMux with the
// generated RegisterClientsServiceHandlerServer, mounted behind the root HTTP
// mux alongside /healthz.
func newClientsGateway(t *testing.T, fake *fakeClientsService) *httptest.Server {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	gatewayMux := runtime.NewServeMux()
	if err := clientsgrpc.RegisterClientsServiceHandlerServer(ctx, gatewayMux, fake); err != nil {
		t.Fatalf("register clients grpc-gateway handler: %v", err)
	}

	httpMux := http.NewServeMux()
	httpMux.Handle("/", gatewayMux)
	httpMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := httptest.NewServer(httpMux)
	t.Cleanup(srv.Close)
	return srv
}

func TestGateway_ClientsRoute_JSONMapping(t *testing.T) {
	srv := newClientsGateway(t, &fakeClientsService{})

	resp, err := http.Get(srv.URL + "/v1/public/clients/cli_123")
	if err != nil {
		t.Fatalf("GET /v1/public/clients/cli_123: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	client, ok := body["client"].(map[string]interface{})
	if !ok {
		t.Fatalf("response field %q missing or not an object: %v", "client", body)
	}

	if got := client["id"]; got != "cli_123" {
		t.Errorf("client.id = %v, want %q", got, "cli_123")
	}
	if got := client["name"]; got != "Acme Corp" {
		t.Errorf("client.name = %v, want %q", got, "Acme Corp")
	}
	if got := client["status"]; got != "CLIENT_STATUS_ACTIVE" {
		t.Errorf("client.status = %v, want %q", got, "CLIENT_STATUS_ACTIVE")
	}
	if got := client["createdAt"]; got != "2026-08-01T00:00:00Z" {
		t.Errorf("client.createdAt = %v, want %q", got, "2026-08-01T00:00:00Z")
	}
}

func TestGateway_SubAccountsRoute(t *testing.T) {
	// Proves the Dashboard route GET /v1/public/clients/sub-accounts reaches
	// the ListSubAccounts RPC (permitted /v1/public/clients* ALB prefix).
	srv := newClientsGateway(t, &fakeClientsService{})

	resp, err := http.Get(srv.URL + "/v1/public/clients/sub-accounts?page=1&pageSize=20")
	if err != nil {
		t.Fatalf("GET /v1/public/clients/sub-accounts: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	rows, ok := body["rows"].([]interface{})
	if !ok || len(rows) != 1 {
		t.Fatalf("expected one sub-account row, got %v", body["rows"])
	}
	// int64 fields encode as JSON strings in protojson (grpc-gateway default).
	if got := body["total"]; got != "1" {
		t.Errorf("total = %v, want \"1\"", got)
	}
}

func TestGateway_SubAccountsRoute_NotOnOldPath(t *testing.T) {
	// The old /v1/public/sub-accounts route must no longer be registered.
	srv := newClientsGateway(t, &fakeClientsService{})

	resp, err := http.Get(srv.URL + "/v1/public/sub-accounts")
	if err != nil {
		t.Fatalf("GET old sub-accounts path: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		t.Fatalf("old route /v1/public/sub-accounts unexpectedly returned %d", resp.StatusCode)
	}
}

func TestGateway_ErrorPropagation(t *testing.T) {
	fake := &fakeClientsService{getClientErr: status.Error(codes.NotFound, "client not found")}
	srv := newClientsGateway(t, fake)

	resp, err := http.Get(srv.URL + "/v1/public/clients/missing")
	if err != nil {
		t.Fatalf("GET /v1/public/clients/missing: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGateway_UnimplementedRPC(t *testing.T) {
	srv := newClientsGateway(t, &fakeClientsService{})

	// CreateClient is not implemented by the fake; the generated
	// UnimplementedClientsServiceServer must map it to HTTP 501.
	resp, err := http.Post(srv.URL+"/v1/public/clients", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /v1/public/clients: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotImplemented)
	}
}

func TestGateway_Healthz(t *testing.T) {
	srv := newClientsGateway(t, &fakeClientsService{})

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	resp, err = http.Post(srv.URL+"/healthz", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("healthz POST status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}
