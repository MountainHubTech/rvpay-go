package health

import (
	"context"
	"testing"

	// Replace "yourproject" with your actual Go module name from go.mod
	"github.com/I-Frostbyte/rvpay-go/grpc/go/clientsgrpc"
)

func TestHealthCheck(t *testing.T) {
	// 1. Initialize your service implementation
	service := &Impl{}

	// 2. Create the context and mock request
	ctx := context.Background()
	req := &clientsgrpc.HealthCheckRequest{}

	// 3. Call the method
	res, err := service.HealthCheck(ctx, req)

	// 4. Assertions
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if res == nil {
		t.Fatal("expected response to not be nil")
	}

	expectedStatus := "ok"
	if res.Status != expectedStatus {
		t.Errorf("expected status %q, got %q", expectedStatus, res.Status)
	}
}
