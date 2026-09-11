package audit

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/alrazihi/civora/internal/audit/domain"
	"github.com/alrazihi/civora/internal/config"
	"github.com/google/uuid"
)

// AuditMaintenanceService runs periodic audit integrity verification and
// retention purging. It is intentionally lightweight: it does not modify
// audit data, only reads it and reports results.
//
// Verification recomputes the SHA-256 hash of every audit event for each
// organization and checks that each event links to its predecessor. Any
// event that fails verification is reported as a verification failure.
// Retention purging deletes events older than the configured retention
// period.
type AuditMaintenanceService struct {
	repo   domain.AuditRepository
	cfg    config.AuditConfig
	logger *log.Logger
}

func NewAuditMaintenanceService(repo domain.AuditRepository, cfg config.AuditConfig) *AuditMaintenanceService {
	return &AuditMaintenanceService{
		repo:   repo,
		cfg:    cfg,
		logger: log.Default(),
	}
}

// VerifyAllOrganizations verifies the audit hash chain for every
// organization that has at least one audit event. It discovers the
// organizations by querying the repository. It returns the total number of
// events verified, the number of events that failed verification, and the
// number of organizations checked. It never returns an error: a
// verification failure is a security incident and is reported via the
// returned counts.
func (s *AuditMaintenanceService) VerifyAllOrganizations(ctx context.Context) (totalVerified, totalFailed, orgsChecked int, err error) {
	if !s.cfg.HashChainEnabled {
		return 0, 0, 0, nil
	}

	orgIDs, err := s.repo.AllOrganizationIDs(ctx)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to list organizations with audit events: %w", err)
	}

	for _, orgID := range orgIDs {
		v, f, err := s.VerifyOrganization(ctx, orgID)
		if err != nil {
			return 0, 0, 0, err
		}
		totalVerified += v
		totalFailed += f
		orgsChecked++
	}

	return totalVerified, totalFailed, orgsChecked, nil
}

// VerifyOrganization verifies the audit hash chain for a single
// organization. It returns the number of events verified, the number of
// events that failed verification, and any error encountered while reading
// events from the repository.
func (s *AuditMaintenanceService) VerifyOrganization(ctx context.Context, orgID uuid.UUID) (verified, failed int, err error) {
	if !s.cfg.HashChainEnabled {
		return 0, 0, nil
	}

	count, err := s.repo.CountByOrganization(ctx, orgID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to count audit events for organization %s: %w", orgID, err)
	}

	events, err := s.repo.FindByOrganization(ctx, orgID, count, 0)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read audit events for organization %s: %w", orgID, err)
	}

	// VerifyChain expects events in chronological order (oldest first), but
	// FindByOrganization returns them newest-first. Reverse before verifying.
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}

	verified, failed, err = domain.VerifyChain(events)
	if err != nil {
		return 0, failed, fmt.Errorf("audit chain verification failed for organization %s: %w", orgID, err)
	}
	if failed > 0 {
		s.logger.Printf("AUDIT INTEGRITY FAILURE: organization %s has %d events that failed verification", orgID, failed)
	}
	return verified, failed, nil
}

// PurgeOld deletes audit events older than the configured retention period.
// Returns the number of events deleted.
func (s *AuditMaintenanceService) PurgeOld(ctx context.Context) (int, error) {
	if s.cfg.RetentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().AddDate(0, 0, -s.cfg.RetentionDays)
	return s.repo.PurgeOld(ctx, cutoff)
}

// RunOnce performs a single verification pass over all organizations with
// audit events and a single retention purge. It is suitable for testing and
// for use as a one-shot maintenance command.
func (s *AuditMaintenanceService) RunOnce(ctx context.Context) (totalVerified, totalFailed, orgsChecked, purged int, err error) {
	verified, failed, orgs, err := s.VerifyAllOrganizations(ctx)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	totalVerified = verified
	totalFailed = failed
	orgsChecked = orgs

	purged, err = s.PurgeOld(ctx)
	if err != nil {
		return totalVerified, totalFailed, orgsChecked, 0, fmt.Errorf("failed to purge old audit events: %w", err)
	}

	return totalVerified, totalFailed, orgsChecked, purged, nil
}

// StartBackground starts a background goroutine that runs the maintenance
// loop on the given interval. It returns a stop channel that, when closed,
// causes the goroutine to exit after the current iteration completes.
//
// The interval must be positive. The background job discovers organizations
// with audit events on each tick by calling AllOrganizationIDs.
func (s *AuditMaintenanceService) StartBackground(ctx context.Context, interval time.Duration) chan struct{} {
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				verified, failed, _, _, err := s.RunOnce(ctx)
				if err != nil {
					s.logger.Printf("audit maintenance error: %v", err)
				}
				if failed > 0 {
					s.logger.Printf("AUDIT INTEGRITY FAILURE: %d of %d events failed verification", failed, verified+failed)
				}
			}
		}
	}()

	return stop
}
