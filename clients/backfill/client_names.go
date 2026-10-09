// Package backfill contains idempotent, application-level data corrections that
// require live provider API calls and therefore cannot be expressed as a SQL
// migration.
//
// The client-name backfill implemented here resolves the authoritative
// HighLevel location/sub-account name for every HighLevel client whose
// clients.display_name is still unset and persists it:
//
//	affected client (display_name IS NULL/empty)
//	  ↓
//	integrations.external_account_id  (authoritative GHL locationId)
//	  ↓
//	stored location token (refreshed once when expired, existing path)
//	  ↓
//	GET /locations/{locationId}   (existing shared v3 client, locations.readonly)
//	  ↓
//	authoritative location name
//	  ↓
//	clients.display_name          (guarded update: fills only a missing name)
//
// Guarantees:
//   - client ids, client_name ("highlevel-<locationId>") and
//     integrations.external_account_id are never modified;
//   - a valid existing display name is never overwritten by this backfill;
//   - nothing is fabricated: a location whose name cannot be fetched keeps its
//     existing value and is reported as an uncorrected record with the exact
//     reason (missing token, 401/403 missing scope, 404, 429, 5xx, timeout,
//     empty name);
//   - an independent failure never aborts the run, and the whole run is safe to
//     repeat: corrected records drop out of the candidate query.
package backfill

import (
	"context"
	"strings"

	"github.com/MountainHubTech/rvpay-go/clients/db/sqlc"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// defaultPageSize is the candidate page size used by the backfill runner.
const defaultPageSize int32 = 100

// ClientLister supplies the backfill's candidate records. repo.ClientRepo
// satisfies it; the interface keeps the runner testable without a database.
type ClientLister interface {
	// ListNeedingDisplayName returns HighLevel clients without a display name.
	ListNeedingDisplayName(ctx context.Context, limit, offset int32) ([]sqlc.ListClientsNeedingDisplayNameRow, error)
	// GetByID re-reads a client to verify the persisted display name.
	GetByID(ctx context.Context, id uuid.UUID) (sqlc.Client, error)
}

// NameSyncer resolves and persists the authoritative location name for a
// locationId. *oauth.Service satisfies it.
type NameSyncer interface {
	SyncClientDisplayName(ctx context.Context, locationID string) error
}

// Failure describes a candidate that could not be corrected automatically.
type Failure struct {
	// ClientID is the RVPay client id (never modified by the backfill).
	ClientID string
	// LocationID is the HighLevel locationId the correction was attempted for.
	LocationID string
	// Reason is the honest, credential-free reason the record was not
	// corrected. It is reported to the operator; it is never written into the
	// client's name.
	Reason string
}

// Report summarises one backfill run. Updated counts only clients whose display
// name was persisted AND verified by a re-read; Failed lists every record that
// still has no authoritative name after the run, with its reason.
type Report struct {
	// Candidates is the number of records examined (dry-run: found).
	Candidates int
	// Updated is the number of records corrected in this run.
	Updated int
	// Failed is the number of records that could not be corrected.
	Failed int
	// Failures carries one entry per uncorrected record.
	Failures []Failure
}

// ClientNamesBackfill resolves and persists client sub-account display names.
type ClientNamesBackfill struct {
	clients  ClientLister
	syncer   NameSyncer
	logger   zerolog.Logger
	pageSize int32
}

// NewClientNamesBackfill creates the backfill runner.
func NewClientNamesBackfill(clients ClientLister, syncer NameSyncer, logger zerolog.Logger) *ClientNamesBackfill {
	return &ClientNamesBackfill{
		clients:  clients,
		syncer:   syncer,
		logger:   logger,
		pageSize: defaultPageSize,
	}
}

// Run executes the backfill. With dryRun=true it only reports the candidates
// that WOULD be corrected and writes nothing (no HighLevel call is made).
//
// The run never aborts on an individual failure: every independent failure is
// recorded in the report and the run continues.
func (b *ClientNamesBackfill) Run(ctx context.Context, dryRun bool) (Report, error) {
	if dryRun {
		return b.reportCandidates(ctx)
	}

	report := Report{}
	// The candidate query only returns clients WITHOUT a display name, so a
	// successfully corrected record leaves the result set and paginating by
	// offset would skip records. The runner therefore always reads the first
	// page and tracks the ids already attempted; it stops as soon as a page
	// adds no new candidate, which also bounds failed records to one attempt
	// per run.
	seen := make(map[uuid.UUID]struct{})
	for {
		rows, err := b.clients.ListNeedingDisplayName(ctx, b.pageSize, 0)
		if err != nil {
			return report, err
		}

		pending := make([]sqlc.ListClientsNeedingDisplayNameRow, 0, len(rows))
		for _, row := range rows {
			if _, done := seen[row.ID]; done {
				continue
			}
			pending = append(pending, row)
		}
		if len(pending) == 0 {
			break
		}

		for _, row := range pending {
			seen[row.ID] = struct{}{}
			b.correctOne(ctx, row, &report)
		}
	}

	return report, nil
}

// reportCandidates lists the records that would be corrected without writing
// anything. No HighLevel call is made, so this is safe to run against
// production at any time.
func (b *ClientNamesBackfill) reportCandidates(ctx context.Context) (Report, error) {
	report := Report{}
	offset := int32(0)
	for {
		rows, err := b.clients.ListNeedingDisplayName(ctx, b.pageSize, offset)
		if err != nil {
			return report, err
		}
		if len(rows) == 0 {
			return report, nil
		}
		for _, row := range rows {
			report.Candidates++
			report.Failures = append(report.Failures, Failure{
				ClientID:   row.ID.String(),
				LocationID: row.ExternalAccountID,
				Reason:     "dry-run: not corrected (no write performed)",
			})
		}
		report.Failed = len(report.Failures)
		offset += int32(len(rows))
	}
}

// correctOne resolves and persists one candidate's display name and records the
// outcome. Independent failures never abort the run.
func (b *ClientNamesBackfill) correctOne(ctx context.Context, row sqlc.ListClientsNeedingDisplayNameRow, report *Report) {
	report.Candidates++

	syncErr := b.syncer.SyncClientDisplayName(ctx, row.ExternalAccountID)

	// Verify against persistence instead of trusting the sync result: the only
	// proof a name was corrected is that the stored display name is no longer
	// empty.
	client, readErr := b.clients.GetByID(ctx, row.ID)
	if readErr != nil {
		b.recordFailure(report, row, "could not verify the persisted client after the correction attempt")
		return
	}
	if strings.TrimSpace(client.DisplayName) != "" {
		report.Updated++
		b.logger.Info().
			Str("client_id", row.ID.String()).
			Str("location_id", row.ExternalAccountID).
			Msg("client display name corrected")
		return
	}

	reason := "HighLevel returned no usable location name"
	if syncErr != nil {
		reason = syncErr.Error()
	}
	b.recordFailure(report, row, reason)
}

// recordFailure appends an uncorrected record to the report. The reason is
// credential-free (it comes from the provider/oauth error classification) and
// is never persisted into the client's name.
func (b *ClientNamesBackfill) recordFailure(report *Report, row sqlc.ListClientsNeedingDisplayNameRow, reason string) {
	report.Failed++
	report.Failures = append(report.Failures, Failure{
		ClientID:   row.ID.String(),
		LocationID: row.ExternalAccountID,
		Reason:     reason,
	})
	b.logger.Warn().
		Str("client_id", row.ID.String()).
		Str("location_id", row.ExternalAccountID).
		Str("reason", reason).
		Msg("client display name could not be corrected; existing value preserved")
}
