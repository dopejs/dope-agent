package secrets_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/secrets"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func TestR37SecretRotationKeepsPriorVersionSnapshot(t *testing.T) {
	manager, sqliteStore, backend := r37Manager(t)
	ctx := context.Background()
	created, err := manager.Create(ctx, secrets.CreateInput{TenantID: "ten_r37_a", SecretRef: "service/token", Value: "old-token"})
	if err != nil {
		t.Fatalf("create secret: %v", err)
	}
	oldVersionID := created.ActiveVersionID

	rotated, err := manager.Rotate(ctx, secrets.RotateInput{TenantID: "ten_r37_a", SecretRef: "service/token", Value: "new-token"})
	if err != nil {
		t.Fatalf("rotate secret: %v", err)
	}
	if rotated.ActiveVersionID == oldVersionID {
		t.Fatal("rotation did not advance active version")
	}
	resolved, err := manager.Resolve(ctx, secrets.ResolveInput{TenantID: "ten_r37_a", SecretRef: "service/token"})
	if err != nil {
		t.Fatalf("resolve rotated secret: %v", err)
	}
	if resolved.Value != "new-token" {
		t.Fatalf("resolved value=%q, want new-token", resolved.Value)
	}
	oldVersion, ok, err := sqliteStore.GetSecretVersion(ctx, "ten_r37_a", oldVersionID)
	if err != nil {
		t.Fatalf("get old version: %v", err)
	}
	if !ok {
		t.Fatal("old version missing after rotation")
	}
	if oldVersion.Status != secrets.SecretVersionStatusSuperseded {
		t.Fatalf("old version status=%q, want superseded", oldVersion.Status)
	}
	oldValue, err := backend.Get(ctx, oldVersion.ValueBackendRef)
	if err != nil {
		t.Fatalf("get old backend value: %v", err)
	}
	if oldValue != "old-token" {
		t.Fatalf("old value=%q, want old-token", oldValue)
	}
}

func TestR37SecretCrossTenantIsolation(t *testing.T) {
	manager, _, _ := r37Manager(t)
	ctx := context.Background()
	if _, err := manager.Create(ctx, secrets.CreateInput{TenantID: "ten_r37_a", SecretRef: "shared/ref", Value: "tenant-a"}); err != nil {
		t.Fatalf("create tenant A: %v", err)
	}
	if _, err := manager.Create(ctx, secrets.CreateInput{TenantID: "ten_r37_b", SecretRef: "shared/ref", Value: "tenant-b"}); err != nil {
		t.Fatalf("create tenant B same ref: %v", err)
	}
	if _, err := manager.Create(ctx, secrets.CreateInput{TenantID: "ten_r37_a", SecretRef: "tenant-a-only", Value: "private-a"}); err != nil {
		t.Fatalf("create tenant A private ref: %v", err)
	}
	resolvedA, err := manager.Resolve(ctx, secrets.ResolveInput{TenantID: "ten_r37_a", SecretRef: "shared/ref"})
	if err != nil {
		t.Fatalf("resolve tenant A: %v", err)
	}
	resolvedB, err := manager.Resolve(ctx, secrets.ResolveInput{TenantID: "ten_r37_b", SecretRef: "shared/ref"})
	if err != nil {
		t.Fatalf("resolve tenant B: %v", err)
	}
	if resolvedA.Value != "tenant-a" || resolvedB.Value != "tenant-b" {
		t.Fatalf("cross-tenant values mixed: A=%q B=%q", resolvedA.Value, resolvedB.Value)
	}
	if _, err := manager.Rotate(ctx, secrets.RotateInput{TenantID: "ten_r37_b", SecretRef: "tenant-a-only", Value: "x"}); !errors.Is(err, secrets.ErrSecretNotFound) {
		t.Fatalf("rotate missing cross-tenant ref err=%v, want ErrSecretNotFound", err)
	}
	if _, err := manager.Disable(ctx, secrets.DisableInput{TenantID: "ten_r37_b", SecretRef: "tenant-a-only", DisabledReason: "test"}); !errors.Is(err, secrets.ErrSecretNotFound) {
		t.Fatalf("disable missing cross-tenant ref err=%v, want ErrSecretNotFound", err)
	}
}

func r37Manager(t *testing.T) (*secrets.Manager, *store.SQLiteStore, *secrets.LocalBackend) {
	t.Helper()
	sqliteStore, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = sqliteStore.Close() })
	backend, err := secrets.NewLocalBackend(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalBackend: %v", err)
	}
	return secrets.NewManager(sqliteStore, backend), sqliteStore, backend
}
