package catalog

import (
	"context"
	"errors"
	"testing"
	"time"
)

type denyReqs struct{ unmet []Requirement }

func (d denyReqs) Unmet(context.Context, string, []Requirement) []Requirement { return d.unmet }

type denyPerms struct{}

func (denyPerms) Allow(context.Context, string, []string) bool { return false }

func sampleItem() CatalogItem {
	now := time.Now().UTC()
	return CatalogItem{
		Kind: ItemKindSkill, Name: "pdf-extract", TrustTier: TrustTierVerified,
		Permissions: []string{"skills.manage"},
		Versions: []Version{
			{Version: "1.0.0", Source: "registry://pdf-extract", PublishedAt: now},
			{Version: "1.1.0", Source: "registry://pdf-extract", Requirements: []Requirement{{Key: "sandbox_backend", Description: "needs a sandbox"}}, PublishedAt: now},
		},
	}
}

// FR-001/FR-002, US1: register + enable a version is tenant-scoped and audited.
func TestCatalogEnable(t *testing.T) {
	m := NewManager("test", nil, nil)
	item, err := m.RegisterItem(sampleItem())
	if err != nil {
		t.Fatalf("RegisterItem: %v", err)
	}
	en, err := m.Enable(context.Background(), "ten_a", item.ItemID, "1.0.0", "op")
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if en.State != EnablementEnabled || en.ActiveVersion != "1.0.0" || len(en.History) != 1 {
		t.Fatalf("enable not recorded: %+v", en)
	}
	if v, ok := m.ActiveVersion(context.Background(), "ten_a", item.ItemID); !ok || v != "1.0.0" {
		t.Fatalf("active version evidence wrong: %q %v", v, ok)
	}
	// Tenant scoping: a different tenant has no enablement.
	if _, ok := m.ActiveVersion(context.Background(), "ten_b", item.ItemID); ok {
		t.Fatal("enablement leaked across tenants")
	}
}

// FR-003: unmet requirements and denied permissions block enablement (fail closed).
func TestCatalogPolicyBlocks(t *testing.T) {
	item := sampleItem()
	mReq := NewManager("test", denyReqs{unmet: []Requirement{{Key: "sandbox_backend"}}}, nil)
	reg, _ := mReq.RegisterItem(item)
	if _, err := mReq.Enable(context.Background(), "ten_a", reg.ItemID, "1.1.0", "op"); !errors.Is(err, ErrRequirementsUnmet) {
		t.Fatalf("unmet requirements should block: %v", err)
	}
	mPerm := NewManager("test", nil, denyPerms{})
	reg2, _ := mPerm.RegisterItem(item)
	if _, err := mPerm.Enable(context.Background(), "ten_a", reg2.ItemID, "1.0.0", "op"); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("denied permission should block: %v", err)
	}
}

// FR-004, US2: rollback restores the prior enabled version, or disables when none.
func TestCatalogRollback(t *testing.T) {
	m := NewManager("test", nil, nil)
	item, _ := m.RegisterItem(sampleItem())
	_, _ = m.Enable(context.Background(), "ten_a", item.ItemID, "1.0.0", "op")
	_, _ = m.Enable(context.Background(), "ten_a", item.ItemID, "1.1.0", "op")

	rolled, err := m.Rollback(context.Background(), "ten_a", item.ItemID, "op")
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rolled.State != EnablementEnabled || rolled.ActiveVersion != "1.0.0" {
		t.Fatalf("rollback did not restore prior version: %+v", rolled)
	}
	// Rolling back again (no further prior) disables safely.
	rolled2, _ := m.Rollback(context.Background(), "ten_a", item.ItemID, "op")
	if rolled2.State != EnablementDisabled {
		t.Fatalf("rollback with no prior should disable: %+v", rolled2)
	}
}

// FR-005: a disabled item has no active version; runtime evidence reflects enabled state.
func TestCatalogActiveVersionGating(t *testing.T) {
	m := NewManager("test", nil, nil)
	item, _ := m.RegisterItem(sampleItem())
	_, _ = m.Enable(context.Background(), "ten_a", item.ItemID, "1.0.0", "op")
	if _, ok := m.ActiveVersion(context.Background(), "ten_a", item.ItemID); !ok {
		t.Fatal("expected active version when enabled")
	}
	_, _ = m.Disable(context.Background(), "ten_a", item.ItemID, "op")
	if _, ok := m.ActiveVersion(context.Background(), "ten_a", item.ItemID); ok {
		t.Fatal("disabled item must not report an active version")
	}
}

// US3: inspect surfaces unmet requirements so a user can see why a skill is unavailable.
func TestCatalogInspectShowsUnmet(t *testing.T) {
	m := NewManager("test", denyReqs{unmet: []Requirement{{Key: "sandbox_backend"}}}, nil)
	item, _ := m.RegisterItem(sampleItem())
	insp, err := m.Inspect(context.Background(), "ten_a", item.ItemID)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if len(insp.UnmetRequirements) == 0 {
		t.Fatalf("inspect should surface unmet requirements: %+v", insp)
	}
}
