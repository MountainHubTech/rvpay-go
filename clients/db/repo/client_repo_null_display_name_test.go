package repo

import (
	"context"
	"testing"

	"github.com/MountainHubTech/rvpay-go/clients/db/sqlc"
	"github.com/google/uuid"
)

// TestClientRepoScansNullDisplayName is the regression test for the HighLevel
// install HTTP 500: a newly created client always has display_name = NULL
// (migration 000006), but sqlc maps the column to a non-null Go string.
// Before the COALESCE in the Client-returning queries, CreateClient's
// RETURNING * failed with "cannot scan NULL into *string" AFTER inserting the
// row, so the stateless OAuth callback returned gRPC Internal and left a
// client without an integration; every retry then failed again in GetByName.
// The test walks the same create -> lookup path the callback uses.
func TestClientRepoScansNullDisplayName(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	r := NewClientRepo(sqlc.New(pool))

	name := "highlevel-test-null-display-" + uuid.NewString()
	created, err := r.Create(ctx, name, sqlc.ClientStatusACTIVE)
	if err != nil {
		t.Fatalf("Create with NULL display_name: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM clients WHERE id = $1", created.ID)
	})
	if created.DisplayName != "" {
		t.Fatalf("Create DisplayName = %q, want empty for a NULL display_name", created.DisplayName)
	}

	byName, err := r.GetByName(ctx, name)
	if err != nil {
		t.Fatalf("GetByName with NULL display_name: %v", err)
	}
	if byName.ID != created.ID {
		t.Fatalf("GetByName returned %s, want %s", byName.ID, created.ID)
	}

	if _, err := r.GetByID(ctx, created.ID); err != nil {
		t.Fatalf("GetByID with NULL display_name: %v", err)
	}
	if _, err := r.UpdateStatus(ctx, created.ID, sqlc.ClientStatusACTIVE); err != nil {
		t.Fatalf("UpdateStatus with NULL display_name: %v", err)
	}
	if _, err := r.List(ctx, 100, 0); err != nil {
		t.Fatalf("List with a NULL display_name row: %v", err)
	}
	if _, err := r.ListActive(ctx, 100, 0); err != nil {
		t.Fatalf("ListActive with a NULL display_name row: %v", err)
	}

	// The guarded name update still treats the coalesced empty value as
	// "no name yet" and fills it.
	updated, err := r.UpdateDisplayName(ctx, created.ID, "Test Location")
	if err != nil {
		t.Fatalf("UpdateDisplayName after NULL: %v", err)
	}
	if updated.DisplayName != "Test Location" {
		t.Fatalf("UpdateDisplayName DisplayName = %q, want %q", updated.DisplayName, "Test Location")
	}
}
