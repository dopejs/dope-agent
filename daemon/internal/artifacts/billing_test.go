package artifacts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestComputerUseArtifactStorageQuotaDeniesBeforeWrite(t *testing.T) {
	t.Parallel()

	ctx, sqliteStore, manager, tenantID := setupArtifactBillingTest(t, 3)
	dataDir := t.TempDir()
	service := NewService(dataDir)
	service.ConfigureBilling(manager, false)

	_, err := service.SaveComputerUseArtifact(ctx, computeruse.ArtifactCaptureRequest{
		RunID:                "run_artifact_denied",
		ComputerUseSessionID: "cus_artifact_denied",
		ComputerUseActionID:  "cua_artifact_denied",
		Kind:                 computeruse.ArtifactKindScreenshot,
		Content:              []byte("12345"),
	})
	if !errors.Is(err, billing.ErrQuotaDenied) {
		t.Fatalf("expected ErrQuotaDenied, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, "artifacts")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected no artifact directory after quota denial, stat err=%v", statErr)
	}
	assertArtifactCounter(t, sqliteStore, tenantID, 0, 0)
}

func TestComputerUseArtifactStorageCommitsActualSmallerThanEstimate(t *testing.T) {
	t.Parallel()

	ctx, sqliteStore, manager, tenantID := setupArtifactBillingTest(t, 10)
	service := NewService(t.TempDir())
	service.ConfigureBilling(manager, false)

	artifact, err := service.SaveComputerUseArtifact(ctx, computeruse.ArtifactCaptureRequest{
		RunID:                "run_artifact_smaller",
		ComputerUseSessionID: "cus_artifact_smaller",
		ComputerUseActionID:  "cua_artifact_smaller",
		Kind:                 computeruse.ArtifactKindScreenshot,
		Content:              []byte("12345"),
		EstimatedByteSize:    8,
	})
	if err != nil {
		t.Fatalf("SaveComputerUseArtifact returned error: %v", err)
	}

	counter := assertArtifactCounter(t, sqliteStore, tenantID, 5, 0)
	reservation := assertArtifactReservation(t, sqliteStore, tenantID, artifact, billing.ReservationStatusCommitted)
	if reservation.QuotaPeriodID != counter.QuotaPeriodID {
		t.Fatalf("reservation period %q does not match counter period %q", reservation.QuotaPeriodID, counter.QuotaPeriodID)
	}
	if reservation.AmountReserved != 8 || reservation.AmountCommitted != 5 || reservation.AmountRefunded != 3 {
		t.Fatalf("expected reserved=8 committed=5 refunded=3, got %+v", reservation)
	}
}

func TestComputerUseArtifactStorageCommitsActualLargerAndDeniesFutureWork(t *testing.T) {
	t.Parallel()

	ctx, sqliteStore, manager, tenantID := setupArtifactBillingTest(t, 6)
	service := NewService(t.TempDir())
	service.ConfigureBilling(manager, false)

	artifact, err := service.SaveComputerUseArtifact(ctx, computeruse.ArtifactCaptureRequest{
		RunID:                "run_artifact_larger",
		ComputerUseSessionID: "cus_artifact_larger",
		ComputerUseActionID:  "cua_artifact_larger",
		Kind:                 computeruse.ArtifactKindScreenshot,
		Content:              []byte("12345678"),
		EstimatedByteSize:    5,
	})
	if err != nil {
		t.Fatalf("SaveComputerUseArtifact returned error: %v", err)
	}
	reservation := assertArtifactReservation(t, sqliteStore, tenantID, artifact, billing.ReservationStatusCommitted)
	if reservation.AmountReserved != 5 || reservation.AmountCommitted != 8 || reservation.AmountRefunded != 0 {
		t.Fatalf("expected reserved=5 committed=8 refunded=0, got %+v", reservation)
	}
	assertArtifactCounter(t, sqliteStore, tenantID, 8, 0)

	_, err = service.SaveComputerUseArtifact(ctx, computeruse.ArtifactCaptureRequest{
		RunID:                "run_artifact_future_denied",
		ComputerUseSessionID: "cus_artifact_future_denied",
		ComputerUseActionID:  "cua_artifact_future_denied",
		Kind:                 computeruse.ArtifactKindScreenshot,
		Content:              []byte("x"),
	})
	if !errors.Is(err, billing.ErrQuotaDenied) {
		t.Fatalf("expected future artifact write to be denied, got %v", err)
	}
}

func TestComputerUseArtifactStorageReservationReleasedOnWriteFailure(t *testing.T) {
	t.Parallel()

	ctx, sqliteStore, manager, tenantID := setupArtifactBillingTest(t, 10)
	dataDir := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(dataDir, []byte("file blocks artifact directory"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	service := NewService(dataDir)
	service.ConfigureBilling(manager, false)

	_, err := service.SaveComputerUseArtifact(ctx, computeruse.ArtifactCaptureRequest{
		RunID:                "run_artifact_write_failure",
		ComputerUseSessionID: "cus_artifact_write_failure",
		ComputerUseActionID:  "cua_artifact_write_failure",
		Kind:                 computeruse.ArtifactKindScreenshot,
		Content:              []byte("12345"),
		EstimatedByteSize:    5,
	})
	if err == nil {
		t.Fatal("expected artifact write failure")
	}
	counter := assertArtifactCounter(t, sqliteStore, tenantID, 0, 0)
	reservation := assertArtifactReservation(t, sqliteStore, tenantID, expectedComputerUseArtifact("cus_artifact_write_failure", []byte("12345")), billing.ReservationStatusReleased)
	if reservation.QuotaPeriodID != counter.QuotaPeriodID || reservation.AmountRefunded != 5 {
		t.Fatalf("expected released reservation with refunded estimate, got %+v", reservation)
	}
}

func setupArtifactBillingTest(t *testing.T, limit int64) (context.Context, *store.SQLiteStore, *billing.Manager, string) {
	t.Helper()

	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	ctx := context.Background()
	if err := sqliteStore.EnsureBillingCatalog(ctx); err != nil {
		t.Fatalf("EnsureBillingCatalog returned error: %v", err)
	}
	tenantID := "ten_artifact_" + strings.NewReplacer("/", "_").Replace(t.Name())
	now := time.Now().UTC()
	if err := sqliteStore.SavePlan(ctx, billing.TenantPlan{
		PlanID:          "plan_" + tenantID,
		TenantID:        tenantID,
		PlanKey:         "finite",
		Status:          billing.PlanStatusActive,
		EnforcementMode: billing.EnforcementModeEnforced,
		EffectiveAt:     now.Add(-time.Minute),
	}); err != nil {
		t.Fatalf("SavePlan returned error: %v", err)
	}
	if err := sqliteStore.SaveQuotaOverride(ctx, billing.QuotaOverride{
		QuotaOverrideID: "override_" + tenantID,
		TenantID:        tenantID,
		Category:        billing.CategoryArtifactStorageBytes,
		Limit:           &limit,
		EffectiveAt:     now.Add(-time.Minute),
		Reason:          "test artifact storage limit",
	}); err != nil {
		t.Fatalf("SaveQuotaOverride returned error: %v", err)
	}
	ctx = tenantctx.WithContext(ctx, identity.TenantContext{TenantID: tenantID, PrincipalID: "prn_" + tenantID})
	return ctx, sqliteStore, billing.NewManager(sqliteStore), tenantID
}

func expectedComputerUseArtifact(sessionID string, content []byte) computeruse.Artifact {
	sum := sha256.Sum256(content)
	artifactID := "cuart_" + hex.EncodeToString(sum[:8])
	return computeruse.Artifact{
		ArtifactID: artifactID,
		StorageKey: filepath.Join("computer-use", sessionID, artifactID),
	}
}

func assertArtifactCounter(t *testing.T, sqliteStore *store.SQLiteStore, tenantID string, committed, reserved int64) billing.UsageCounter {
	t.Helper()

	definition, ok := billing.DefinitionFor(billing.CategoryArtifactStorageBytes)
	if !ok {
		t.Fatal("artifact storage quota definition missing")
	}
	period, err := sqliteStore.OpenPeriod(context.Background(), tenantID, definition, time.Now().UTC())
	if err != nil {
		t.Fatalf("OpenPeriod returned error: %v", err)
	}
	counter, ok, err := sqliteStore.UsageCounter(context.Background(), tenantID, billing.CategoryArtifactStorageBytes, period.QuotaPeriodID)
	if err != nil {
		t.Fatalf("UsageCounter returned error: %v", err)
	}
	if !ok {
		counter = billing.UsageCounter{}
	}
	if counter.CommittedAmount != committed || counter.ReservedAmount != reserved {
		t.Fatalf("expected counter committed=%d reserved=%d, got %+v", committed, reserved, counter)
	}
	return counter
}

func assertArtifactReservation(t *testing.T, sqliteStore *store.SQLiteStore, tenantID string, artifact computeruse.Artifact, status billing.ReservationStatus) billing.UsageReservation {
	t.Helper()

	operationKey := billing.ArtifactOperationKey(tenantID, artifact.ArtifactID, artifact.StorageKey, "")
	reservation, ok, err := sqliteStore.ReservationByOperation(context.Background(), tenantID, billing.CategoryArtifactStorageBytes, operationKey)
	if err != nil {
		t.Fatalf("ReservationByOperation returned error: %v", err)
	}
	if !ok {
		t.Fatalf("reservation %q not found", operationKey)
	}
	if reservation.Status != status {
		t.Fatalf("expected reservation status %s, got %+v", status, reservation)
	}
	return reservation
}
