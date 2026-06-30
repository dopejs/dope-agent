package catalog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

var (
	ErrItemNotFound       = errors.New("catalog item not found")
	ErrVersionNotFound    = errors.New("catalog item version not found")
	ErrRequirementsUnmet  = errors.New("catalog item requirements are not met")
	ErrPermissionDenied   = errors.New("tenant is not permitted to enable this catalog item")
	ErrNoRollbackTarget   = errors.New("no prior enabled version to roll back to")
	ErrInvalidCatalogItem = errors.New("catalog item definition is invalid")
)

// RequirementChecker reports which of an item version's declared requirements are unmet in the
// given tenant/environment. It is the policy hook that fails closed before install/execution.
type RequirementChecker interface {
	Unmet(ctx context.Context, tenantID string, requirements []Requirement) []Requirement
}

// PermissionGate reports whether a tenant may enable an item requiring the given permissions.
type PermissionGate interface {
	Allow(ctx context.Context, tenantID string, permissions []string) bool
}

type allowAllPermissions struct{}

func (allowAllPermissions) Allow(context.Context, string, []string) bool { return true }

type allMet struct{}

func (allMet) Unmet(context.Context, string, []Requirement) []Requirement { return nil }

// Manager owns the operator catalog and per-tenant enablement. Items + enablements are in-memory
// with Restore for this slice; existing skill/MCP registries can feed catalog projections.
type Manager struct {
	mu          sync.RWMutex
	env         string
	checker     RequirementChecker
	permissions PermissionGate
	items       map[string]CatalogItem
	enablements map[string]Enablement // key: tenantID + "\x00" + itemID
}

func NewManager(environmentScope string, checker RequirementChecker, permissions PermissionGate) *Manager {
	if checker == nil {
		checker = allMet{}
	}
	if permissions == nil {
		permissions = allowAllPermissions{}
	}
	return &Manager{
		env:         strings.TrimSpace(environmentScope),
		checker:     checker,
		permissions: permissions,
		items:       make(map[string]CatalogItem),
		enablements: make(map[string]Enablement),
	}
}

func enablementKey(tenantID, itemID string) string {
	return strings.TrimSpace(tenantID) + "\x00" + strings.TrimSpace(itemID)
}

// RegisterItem inserts or replaces an operator-curated catalog item (a projection an operator
// curates; the agent never authors items here).
func (m *Manager) RegisterItem(item CatalogItem) (CatalogItem, error) {
	if strings.TrimSpace(item.Name) == "" || !validKind(item.Kind) || len(item.Versions) == 0 {
		return CatalogItem{}, ErrInvalidCatalogItem
	}
	now := time.Now().UTC()
	if strings.TrimSpace(item.ItemID) == "" {
		item.ItemID = newID("catalog_item")
		item.CreatedAt = now
	}
	if item.TrustTier == "" {
		item.TrustTier = TrustTierCommunity
	}
	item.UpdatedAt = now
	m.mu.Lock()
	if existing, ok := m.items[item.ItemID]; ok && !existing.CreatedAt.IsZero() {
		item.CreatedAt = existing.CreatedAt
	}
	m.items[item.ItemID] = item
	m.mu.Unlock()
	return item, nil
}

func (m *Manager) GetItem(itemID string) (CatalogItem, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.items[strings.TrimSpace(itemID)]
	return item, ok
}

func (m *Manager) ListItems() []CatalogItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]CatalogItem, 0, len(m.items))
	for _, item := range m.items {
		out = append(out, item)
	}
	return out
}

// Enable enables a catalog item version for a tenant after the permission and requirement gates
// pass (fail closed). Enablement is recorded with an audit event.
func (m *Manager) Enable(ctx context.Context, tenantID, itemID, version, actor string) (Enablement, error) {
	item, ok := m.GetItem(itemID)
	if !ok {
		return Enablement{}, ErrItemNotFound
	}
	target, ok := resolveVersion(item, version)
	if !ok {
		return Enablement{}, ErrVersionNotFound
	}
	if !m.permissions.Allow(ctx, tenantID, item.Permissions) {
		return Enablement{}, ErrPermissionDenied
	}
	if unmet := m.checker.Unmet(ctx, tenantID, target.Requirements); len(unmet) > 0 {
		return Enablement{}, ErrRequirementsUnmet
	}
	return m.recordTransition(tenantID, itemID, EnablementEnabled, target.Version, "enabled", actor, ""), nil
}

// Disable disables a catalog item for a tenant.
func (m *Manager) Disable(ctx context.Context, tenantID, itemID, actor string) (Enablement, error) {
	if _, ok := m.GetItem(itemID); !ok {
		return Enablement{}, ErrItemNotFound
	}
	return m.recordTransition(tenantID, itemID, EnablementDisabled, "", "disabled", actor, ""), nil
}

// Rollback restores the prior enabled version from the audit history, or disables safely when
// there is no prior version (FR-004).
func (m *Manager) Rollback(ctx context.Context, tenantID, itemID, actor string) (Enablement, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := enablementKey(tenantID, itemID)
	enablement, ok := m.enablements[key]
	if !ok {
		return Enablement{}, ErrNoRollbackTarget
	}
	now := time.Now().UTC()
	// Pop the current active version off the stack; the new top (if any) is restored.
	if len(enablement.VersionStack) > 0 {
		enablement.VersionStack = enablement.VersionStack[:len(enablement.VersionStack)-1]
	}
	if len(enablement.VersionStack) == 0 {
		enablement.State = EnablementDisabled
		enablement.ActiveVersion = ""
		enablement.History = append(enablement.History, EnablementEvent{Action: "rolled_back", Actor: actor, Reason: "no prior version; disabled", OccurredAt: now})
	} else {
		prior := enablement.VersionStack[len(enablement.VersionStack)-1]
		enablement.State = EnablementEnabled
		enablement.ActiveVersion = prior
		enablement.History = append(enablement.History, EnablementEvent{Action: "rolled_back", Version: prior, Actor: actor, OccurredAt: now})
	}
	enablement.UpdatedAt = now
	m.enablements[key] = enablement
	return enablement, nil
}

// Inspect returns the item, the tenant's enablement, unmet requirements for the active/latest
// version, and whether the permission gate is satisfied (FR-005, US3).
func (m *Manager) Inspect(ctx context.Context, tenantID, itemID string) (Inspection, error) {
	item, ok := m.GetItem(itemID)
	if !ok {
		return Inspection{}, ErrItemNotFound
	}
	m.mu.RLock()
	enablement := m.enablements[enablementKey(tenantID, itemID)]
	m.mu.RUnlock()
	target := enablement.ActiveVersion
	version, found := resolveVersion(item, target)
	if !found {
		version, _ = item.latest()
	}
	return Inspection{
		Item:                item,
		Enablement:          enablement,
		UnmetRequirements:   m.checker.Unmet(ctx, tenantID, version.Requirements),
		PermissionSatisfied: m.permissions.Allow(ctx, tenantID, item.Permissions),
	}, nil
}

// ActiveVersion returns the version influencing execution for a tenant's item, plus whether it is
// currently enabled and requirements remain met (runtime evidence, FR-005).
func (m *Manager) ActiveVersion(ctx context.Context, tenantID, itemID string) (string, bool) {
	m.mu.RLock()
	enablement, ok := m.enablements[enablementKey(tenantID, itemID)]
	m.mu.RUnlock()
	if !ok || enablement.State != EnablementEnabled {
		return "", false
	}
	item, found := m.GetItem(itemID)
	if !found {
		return "", false
	}
	version, vok := resolveVersion(item, enablement.ActiveVersion)
	if !vok {
		return "", false
	}
	if len(m.checker.Unmet(ctx, tenantID, version.Requirements)) > 0 {
		return "", false // requirements regressed; not safe to execute
	}
	return enablement.ActiveVersion, true
}

// Restore reloads persisted items + enablements.
func (m *Manager) Restore(items []CatalogItem, enablements []Enablement) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make(map[string]CatalogItem, len(items))
	for _, item := range items {
		m.items[item.ItemID] = item
	}
	m.enablements = make(map[string]Enablement, len(enablements))
	for _, e := range enablements {
		m.enablements[enablementKey(e.TenantID, e.ItemID)] = e
	}
}

func (m *Manager) recordTransition(tenantID, itemID string, state EnablementState, version, action, actor, reason string) Enablement {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := enablementKey(tenantID, itemID)
	enablement, ok := m.enablements[key]
	if !ok {
		enablement = Enablement{TenantID: strings.TrimSpace(tenantID), ItemID: strings.TrimSpace(itemID)}
	}
	now := time.Now().UTC()
	enablement.State = state
	if state == EnablementEnabled {
		enablement.ActiveVersion = version
		// Push onto the version stack (dedup the current top) for deterministic rollback.
		if n := len(enablement.VersionStack); n == 0 || enablement.VersionStack[n-1] != version {
			enablement.VersionStack = append(enablement.VersionStack, version)
		}
	} else {
		enablement.ActiveVersion = ""
		enablement.VersionStack = nil
	}
	enablement.History = append(enablement.History, EnablementEvent{Action: action, Version: version, Actor: actor, Reason: reason, OccurredAt: now})
	enablement.UpdatedAt = now
	m.enablements[key] = enablement
	return enablement
}

func resolveVersion(item CatalogItem, version string) (Version, bool) {
	if strings.TrimSpace(version) == "" {
		return item.latest()
	}
	return item.version(version)
}

func validKind(k ItemKind) bool {
	switch k {
	case ItemKindSkill, ItemKindMCPServer, ItemKindCapability:
		return true
	default:
		return false
	}
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_fallback"
	}
	return prefix + "_" + hex.EncodeToString(buf)
}
