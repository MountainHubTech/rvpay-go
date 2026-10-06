package repo

import (
	"context"
	"errors"
	"testing"

	"github.com/MountainHubTech/rvpay-go/clients/db/sqlc"
	"github.com/google/uuid"
)

// Regression suite for the nullable clients.display_name (migration 000006)
// NULL-scan defect that aborted HighLevel OAuth installation: every repository
// query returning display_name scans it into the non-null Go string
// sqlc.Client.DisplayName, so a SQL NULL used to fail pgx with "cannot scan
// NULL into *string" and surface as gRPC Internal / HTTP 500. DB-backed like
// client_repo_subaccounts_test.go (skipped unless CLIENTS_TEST_DATABASE_URL is
// set); synthetic clients use NULL display_name values on purpose and say
// nothing about any production/testing location.

// Scenario C (primary OAuth regression): CreateClient never sets display_name,
// so its RETURNING row carries NULL.
func TestCreateClientWithoutDisplayNameSucceeds(t *testing.T) {
	pool := newTestPool(t)
	repo := NewClientRepo(sqlc.New(pool))
	ctx := context.Background()

	const name = "highlevel-nullscan-create"
	client, err := repo.Create(ctx, name, sqlc.ClientStatusACTIVE)
	if err != nil {
		t.Fatalf("Create must succeed when display_name is NULL: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(ctx, client.ID) })

	if client.ClientName != name {
		t.Errorf("client_name = %q, want %q", client.ClientName, name)
	}
	if client.DisplayName != name {
		t.Errorf("display_name = %q, want client_name fallback %q", client.DisplayName, name)
	}
}

// Scenario D: an existing NULL-display_name client is retrievable through the
// OAuth-install lookup (GetByName) and by id, without a scan error.
func TestGetByNameAndByIDScanNullDisplayName(t *testing.T) {
	pool := newTestPool(t)
	repo := NewClientRepo(sqlc.New(pool))
	ctx := context.Background()

	const name = "highlevel-nullscan-lookup"
	id := uuid.MustParse(insertSubAccountFixture(t, pool, name, nil))

	byName, err := repo.GetByName(ctx, name)
	if err != nil {
		t.Fatalf("GetByName must succeed when display_name IS NULL: %v", err)
	}
	if byName.DisplayName != name {
		t.Errorf("GetByName display_name = %q, want %q", byName.DisplayName, name)
	}

	byID, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID must succeed when display_name IS NULL: %v", err)
	}
	if byID.DisplayName != name {
		t.Errorf("GetByID display_name = %q, want %q", byID.DisplayName, name)
	}
}

// Scenario B: a non-null friendly display name is preserved unchanged.
func TestGetByIDPreservesFriendlyDisplayName(t *testing.T) {
	pool := newTestPool(t)
	repo := NewClientRepo(sqlc.New(pool))
	ctx := context.Background()

	friendly := "RVPay Test Location"
	id := uuid.MustParse(insertSubAccountFixture(t, pool, "highlevel-nullscan-friendly", &friendly))

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.DisplayName != friendly {
		t.Errorf("display_name = %q, want friendly name %q preserved", got.DisplayName, friendly)
	}
}

// Scenario E: NULL and non-NULL display names in the same ListClients result
// set all scan, with the fallback applied only to the NULL row.
func TestListClientsScansMixedDisplayNameValues(t *testing.T) {
	pool := newTestPool(t)
	repo := NewClientRepo(sqlc.New(pool))
	ctx := context.Background()

	friendlyA := "Friendly A"
	friendlyC := "Friendly C"
	insertSubAccountFixture(t, pool, "highlevel-nullscan-multi-a", &friendlyA)
	insertSubAccountFixture(t, pool, "highlevel-nullscan-multi-b", nil)
	insertSubAccountFixture(t, pool, "highlevel-nullscan-multi-c", &friendlyC)

	rows, err := repo.List(ctx, 100, 0)
	if err != nil {
		t.Fatalf("List must not fail when a display_name IS NULL: %v", err)
	}
	want := map[string]string{
		"highlevel-nullscan-multi-a": "Friendly A",
		"highlevel-nullscan-multi-b": "highlevel-nullscan-multi-b",
		"highlevel-nullscan-multi-c": "Friendly C",
	}
	seen := make(map[string]string, len(want))
	for _, row := range rows {
		if _, ok := want[row.ClientName]; ok {
			seen[row.ClientName] = row.DisplayName
		}
	}
	for name, wantDisplay := range want {
		got, ok := seen[name]
		if !ok {
			t.Errorf("client %q missing from ListClients result", name)
			continue
		}
		if got != wantDisplay {
			t.Errorf("client %q display_name = %q, want %q", name, got, wantDisplay)
		}
	}
}

// Scenario F: the repository sequence the OAuth callback performs for a fresh
// location — GetByName miss → CreateClient → GetByName hit — completes without
// a NULL-scan abort.
func TestOAuthInstallClientResolutionSequence(t *testing.T) {
	pool := newTestPool(t)
	repo := NewClientRepo(sqlc.New(pool))
	ctx := context.Background()

	const name = "highlevel-nullscan-oauth-path"
	if _, err := repo.GetByName(ctx, name); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByName miss = %v, want ErrNotFound", err)
	}

	created, err := repo.Create(ctx, name, sqlc.ClientStatusACTIVE)
	if err != nil {
		t.Fatalf("CreateClient must succeed on the OAuth path: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(ctx, created.ID) })

	resolved, err := repo.GetByName(ctx, name)
	if err != nil {
		t.Fatalf("GetByName after create failed: %v", err)
	}
	if resolved.ID != created.ID {
		t.Errorf("resolved id = %s, want %s", resolved.ID, created.ID)
	}
	if resolved.DisplayName != name {
		t.Errorf("resolved display_name = %q, want %q", resolved.DisplayName, name)
	}
}

// UpdateClientStatus also RETURNs display_name and must scan NULL safely.
func TestUpdateStatusScansNullDisplayName(t *testing.T) {
	pool := newTestPool(t)
	repo := NewClientRepo(sqlc.New(pool))
	ctx := context.Background()

	const name = "highlevel-nullscan-update-status"
	id := uuid.MustParse(insertSubAccountFixture(t, pool, name, nil))

	updated, err := repo.UpdateStatus(ctx, id, sqlc.ClientStatusSUSPENDED)
	if err != nil {
		t.Fatalf("UpdateStatus must succeed when display_name IS NULL: %v", err)
	}
	if updated.DisplayName != name {
		t.Errorf("display_name = %q, want %q", updated.DisplayName, name)
	}
}

// UpdateClientDisplayName RETURNs display_name too; the guarded fill persists
// the friendly name and never overwrites a populated one.
func TestUpdateDisplayNamePersistsFriendlyName(t *testing.T) {
	pool := newTestPool(t)
	repo := NewClientRepo(sqlc.New(pool))
	ctx := context.Background()

	const name = "highlevel-nullscan-update-display"
	id := uuid.MustParse(insertSubAccountFixture(t, pool, name, nil))

	updated, err := repo.UpdateDisplayName(ctx, id, "Fresh Location Name")
	if err != nil {
		t.Fatalf("UpdateDisplayName failed: %v", err)
	}
	if updated.DisplayName != "Fresh Location Name" {
		t.Errorf("display_name = %q, want %q", updated.DisplayName, "Fresh Location Name")
	}

	// Guard: a second fill of a populated name returns no rows → ErrNotFound.
	if _, err := repo.UpdateDisplayName(ctx, id, "Other Name"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second UpdateDisplayName = %v, want ErrNotFound", err)
	}
}
