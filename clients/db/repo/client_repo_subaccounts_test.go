package repo

import (
	"context"
	"os"
	"testing"

	"github.com/MountainHubTech/rvpay-go/clients/db/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newTestPool opens a connection to the disposable test database. The tests are
// skipped when CLIENTS_TEST_DATABASE_URL is not set so the regular unit-test
// suite stays hermetic.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("CLIENTS_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CLIENTS_TEST_DATABASE_URL not set; skipping DB-backed repository test")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// insertSubAccountFixture inserts a clients row with an explicit display_name
// (nil means SQL NULL) and returns its id. The row is removed at test end.
// The insert uses plain SQL on purpose: the generated CreateClient/RETURNING *
// helpers scan the nullable display_name column themselves, which is a separate
// code path from the Sub-Accounts listing this regression covers.
func insertSubAccountFixture(t *testing.T, pool *pgxpool.Pool, clientName string, displayName *string) string {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	_, err := pool.Exec(ctx,
		"INSERT INTO clients (id, client_name, status, display_name) VALUES ($1, $2, 'ACTIVE', $3)",
		id, clientName, displayName)
	if err != nil {
		t.Fatalf("create client fixture %q: %v", clientName, err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM clients WHERE id = $1", id)
	})
	return id.String()
}

// TestListSubAccountsScansNullDisplayName is the regression test for the
// Sub-Accounts HTTP 500: clients.display_name is a NULLABLE column
// (migration 000006), but the repository result type scans it into a non-null
// Go string. Before the COALESCE in ListSubAccountsFiltered, this row made
// rows.Scan fail with "cannot scan NULL into *string" and the endpoint
// returned gRPC Internal / HTTP 500. The test asserts the row now scans and
// resolves to the deterministic highlevel-<locationId> fallback.
func TestListSubAccountsScansNullDisplayName(t *testing.T) {
	pool := newTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	insertSubAccountFixture(t, pool, "highlevel-location-a", nil)

	rows, err := NewClientRepo(q).ListSubAccounts(ctx, "highlevel-location-a", "", "", "", 20, 0)
	if err != nil {
		t.Fatalf("ListSubAccounts must not fail when display_name IS NULL: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if got := rows[0].DisplayName; got != "highlevel-location-a" {
		t.Errorf("display_name = %q, want highlevel-location-a", got)
	}
}

// TestListSubAccountsPreservesPopulatedDisplayName proves a non-null friendly
// name keeps its meaning: the fallback must not overwrite it.
func TestListSubAccountsPreservesPopulatedDisplayName(t *testing.T) {
	pool := newTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	friendly := "Friendly Account"
	insertSubAccountFixture(t, pool, "highlevel-location-b", &friendly)

	rows, err := NewClientRepo(q).ListSubAccounts(ctx, "highlevel-location-b", "", "", "", 20, 0)
	if err != nil {
		t.Fatalf("ListSubAccounts failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if got := rows[0].DisplayName; got != "Friendly Account" {
		t.Errorf("display_name = %q, want Friendly Account", got)
	}
}

// TestListSubAccountsMultipleAccountsKeepRows proves a NULL display name no
// longer removes an account from the response: friendly, NULL and friendly
// rows all survive the scan.
func TestListSubAccountsMultipleAccountsKeepRows(t *testing.T) {
	pool := newTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	friendlyA := "Friendly A"
	friendlyC := "Friendly C"
	insertSubAccountFixture(t, pool, "highlevel-multi-a", &friendlyA)
	insertSubAccountFixture(t, pool, "highlevel-multi-b", nil)
	insertSubAccountFixture(t, pool, "highlevel-multi-c", &friendlyC)

	rows, err := NewClientRepo(q).ListSubAccounts(ctx, "highlevel-multi-", "", "name", "asc", 20, 0)
	if err != nil {
		t.Fatalf("ListSubAccounts must not fail when a display_name IS NULL: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	// Sorted by name ascending: "Friendly C" sorts before the lower-case
	// highlevel- fallback under the database collation. The point of the
	// assertion is that the NULL display_name row is present and resolved,
	// not the collation order of the letters.
	want := []string{"Friendly A", "Friendly C", "highlevel-multi-b"}
	for i, w := range want {
		if got := rows[i].DisplayName; got != w {
			t.Errorf("row %d display_name = %q, want %q", i, got, w)
		}
	}
}
