package repo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/MountainHubTech/rvpay-go/transactions/db/sqlc"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newTestPool opens a connection to the disposable Transactions test database.
// The tests are skipped when TRANSACTIONS_TEST_DATABASE_URL is not set so the
// regular unit-test suite stays hermetic (same convention as the Clients
// CLIENTS_TEST_DATABASE_URL repository tests).
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TRANSACTIONS_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TRANSACTIONS_TEST_DATABASE_URL not set; skipping DB-backed repository test")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// insertDepositFixture inserts a deposits row for the given client with an
// explicit created_at (for deterministic ordering) and returns its id. The
// row is removed at test end. The insert uses plain SQL on purpose (same
// convention as the Clients sub-account fixtures): it exercises the exact
// schema produced by the migrations, not the generated Create helpers.
func insertDepositFixture(t *testing.T, pool *pgxpool.Pool, clientName, status string, createdAt time.Time) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO deposits (client_name, amount, currency, payment_type, payer_phone_number, provider, status, idempotency_key, initiated_at, created_at)
		 VALUES ($1, 100.00, 'XAF', 'MMO', '+237600000000', 'MTN_MOMO', $2::deposit_status, gen_random_uuid(), $3, $3)
		 RETURNING id`,
		clientName, status, createdAt).Scan(&id)
	if err != nil {
		t.Fatalf("create deposit fixture for %q: %v", clientName, err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM deposits WHERE id = $1", id)
	})
	return id
}

// TestListFilteredIsolatesSubAccounts is the mandatory cross-sub-account
// isolation proof at the SQL layer: three clients (A, B, C) each own
// deposits, and a filtered query for one client must return only that
// client's rows (and only its count) — never another client's transactions.
// This is exactly the filter the extended ListTransactions endpoint passes
// for client_id / location_id / sub_account.
func TestListFilteredIsolatesSubAccounts(t *testing.T) {
	pool := newTestPool(t)
	repo := NewDepositRepo(sqlc.New(pool))
	ctx := context.Background()
	base := time.Now().UTC().Add(-24 * time.Hour)

	clients := []string{"highlevel-iso-loc-a", "highlevel-iso-loc-b", "highlevel-iso-loc-c"}
	for i, name := range clients {
		insertDepositFixture(t, pool, name, "COMPLETED", base.Add(time.Duration(i)*time.Minute))
		insertDepositFixture(t, pool, name, "FAILED", base.Add(time.Duration(i)*time.Minute+30*time.Second))
		// Another tenant's activity must never bleed into this tenant's page.
		insertDepositFixture(t, pool, name+"-noise", "COMPLETED", base.Add(time.Duration(i)*time.Minute+45*time.Second))
	}

	for _, name := range clients {
		rows, err := repo.ListFiltered(ctx, "", "", name, 100, 0)
		if err != nil {
			t.Fatalf("ListFiltered(%q) failed: %v", name, err)
		}
		if len(rows) != 2 {
			t.Fatalf("ListFiltered(%q) rows = %d, want exactly 2", name, len(rows))
		}
		for _, row := range rows {
			if row.ClientName != name {
				t.Errorf("tenant leak: filter %q returned row for %q", name, row.ClientName)
			}
		}
		total, err := repo.CountFiltered(ctx, "", "", name)
		if err != nil {
			t.Fatalf("CountFiltered(%q) failed: %v", name, err)
		}
		if total != 2 {
			t.Errorf("CountFiltered(%q) = %d, want 2", name, total)
		}
	}
}

// TestListFilteredUnknownSubAccountReturnsEmpty proves an unknown sub-account
// identifier (an unknown locationId mapped to highlevel-<locationId>) yields
// an empty result — never another tenant's rows.
func TestListFilteredUnknownSubAccountReturnsEmpty(t *testing.T) {
	pool := newTestPool(t)
	repo := NewDepositRepo(sqlc.New(pool))
	ctx := context.Background()

	insertDepositFixture(t, pool, "highlevel-known", "COMPLETED", time.Now().UTC())

	rows, err := repo.ListFiltered(ctx, "", "", "highlevel-unknown-location", 20, 0)
	if err != nil {
		t.Fatalf("ListFiltered failed: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %d, want 0 for unknown sub-account", len(rows))
	}
	total, err := repo.CountFiltered(ctx, "", "", "highlevel-unknown-location")
	if err != nil {
		t.Fatalf("CountFiltered failed: %v", err)
	}
	if total != 0 {
		t.Errorf("count = %d, want 0 for unknown sub-account", total)
	}
}

// TestListFilteredPaginationAndOrdering proves the sub-account filter
// preserves the deterministic created_at DESC ordering and correct
// LIMIT/OFFSET pagination plus total count.
func TestListFilteredPaginationAndOrdering(t *testing.T) {
	pool := newTestPool(t)
	repo := NewDepositRepo(sqlc.New(pool))
	ctx := context.Background()

	client := "highlevel-page-loc"
	now := time.Now().UTC()
	// Insert out of chronological order to prove SQL ordering, not insert
	// order, decides the page.
	oldest := insertDepositFixture(t, pool, client, "COMPLETED", now.Add(-3*time.Hour))
	newest := insertDepositFixture(t, pool, client, "COMPLETED", now)
	middle := insertDepositFixture(t, pool, client, "COMPLETED", now.Add(-1*time.Hour))

	// Full list: newest first.
	rows, err := repo.ListFiltered(ctx, "", "", client, 10, 0)
	if err != nil {
		t.Fatalf("ListFiltered failed: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	wantOrder := []string{newest, middle, oldest}
	for i, wantID := range wantOrder {
		if rows[i].ID.String() != wantID {
			t.Errorf("row %d id = %s, want %s (created_at DESC violated)", i, rows[i].ID, wantID)
		}
	}

	// Page 1 (size 2): the two newest.
	page1, err := repo.ListFiltered(ctx, "", "", client, 2, 0)
	if err != nil {
		t.Fatalf("ListFiltered page 1 failed: %v", err)
	}
	if len(page1) != 2 || page1[0].ID.String() != newest || page1[1].ID.String() != middle {
		t.Errorf("page 1 = %v, want [newest middle]", idsOf(page1))
	}

	// Page 2 (size 2): the oldest.
	page2, err := repo.ListFiltered(ctx, "", "", client, 2, 2)
	if err != nil {
		t.Fatalf("ListFiltered page 2 failed: %v", err)
	}
	if len(page2) != 1 || page2[0].ID.String() != oldest {
		t.Errorf("page 2 = %v, want [oldest]", idsOf(page2))
	}

	total, err := repo.CountFiltered(ctx, "", "", client)
	if err != nil {
		t.Fatalf("CountFiltered failed: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3 (total must count the whole filtered set, not the page)", total)
	}
}

// TestListFilteredPreservesExistingFilters proves the sub-account filter
// composes with the pre-existing search and status filters rather than
// replacing them.
func TestListFilteredPreservesExistingFilters(t *testing.T) {
	pool := newTestPool(t)
	repo := NewDepositRepo(sqlc.New(pool))
	ctx := context.Background()

	client := "highlevel-combo-loc"
	now := time.Now().UTC()
	insertDepositFixture(t, pool, client, "COMPLETED", now.Add(-2*time.Minute))
	insertDepositFixture(t, pool, client, "COMPLETED", now.Add(-1*time.Minute))
	insertDepositFixture(t, pool, client, "FAILED", now)

	// Status + sub-account combined.
	rows, err := repo.ListFiltered(ctx, "", "COMPLETED", client, 20, 0)
	if err != nil {
		t.Fatalf("ListFiltered failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("COMPLETED rows = %d, want 2", len(rows))
	}
	for _, row := range rows {
		if row.ClientName != client || row.Status != sqlc.DepositStatusCOMPLETED {
			t.Errorf("row = (%q, %s), want (%q, COMPLETED)", row.ClientName, row.Status, client)
		}
	}

	// search + sub-account combined (search matches this client's rows only).
	rows, err = repo.ListFiltered(ctx, client, "", client, 20, 0)
	if err != nil {
		t.Fatalf("ListFiltered with search failed: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("search+sub_account rows = %d, want 3", len(rows))
	}

	// search that matches another tenant yields nothing for this client.
	rows, err = repo.ListFiltered(ctx, "highlevel-iso-loc-a", "", client, 20, 0)
	if err != nil {
		t.Fatalf("ListFiltered with foreign search failed: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %d, want 0 when search matches only another tenant", len(rows))
	}
}

// idsOf renders deposit ids for failure messages.
func idsOf(rows []sqlc.Deposit) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ID.String())
	}
	return out
}
