package app

import (
	"context"
	"errors"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/telemetry"
)

func TestGuardTenantMigrationStartupCleanState(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	bus := events.NewBus()
	logger := telemetry.New("info")

	gate, err := guardTenantMigrationStartup(context.Background(), s, logger, bus)
	if err != nil {
		t.Fatalf("guardTenantMigrationStartup on clean store should pass, got: %v", err)
	}
	// A fresh store has the events backfill rows registered as
	// `pending` (T016 + Phase 3 pre-registration), so the gate
	// MUST report InProgress=true so handlers refuse tenant traffic
	// until the backfill driver completes them.
	if !gate.InProgress() {
		t.Fatalf("clean store with pending backfill rows must mark gate InProgress=true; got false")
	}
}

func TestGuardTenantMigrationStartupRefusesOnFailedStep(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	const stepName = "tenant_migration:backfill:runs"
	if err := s.RegisterMigrationStep(ctx, stepName); err != nil {
		t.Fatalf("RegisterMigrationStep: %v", err)
	}
	if _, err := s.BeginMigrationStep(ctx, stepName); err != nil {
		t.Fatalf("BeginMigrationStep: %v", err)
	}
	if err := s.FailMigrationStep(ctx, stepName, errors.New("disk full")); err != nil {
		t.Fatalf("FailMigrationStep: %v", err)
	}

	bus := events.NewBus()
	logger := telemetry.New("info")

	_, err = guardTenantMigrationStartup(ctx, s, logger, bus)
	if !errors.Is(err, ErrTenantMigrationFailed) {
		t.Fatalf("expected ErrTenantMigrationFailed, got %v", err)
	}
}

func TestGuardTenantMigrationStartupResumesRunning(t *testing.T) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	const stepName = "tenant_migration:backfill:schedules"
	if err := s.RegisterMigrationStep(ctx, stepName); err != nil {
		t.Fatalf("RegisterMigrationStep: %v", err)
	}
	if _, err := s.BeginMigrationStep(ctx, stepName); err != nil {
		t.Fatalf("BeginMigrationStep: %v", err)
	}

	gate, err := guardTenantMigrationStartup(ctx, s, telemetry.New("info"), events.NewBus())
	if err != nil {
		t.Fatalf("guard should tolerate running steps, got %v", err)
	}
	if !gate.InProgress() {
		t.Fatalf("running step MUST mark gate InProgress=true so tenant routes 503 until completion")
	}
	pending := gate.PendingSteps()
	if len(pending) == 0 {
		t.Fatalf("gate must surface pending/running step names so the 503 body is correlatable")
	}
}
