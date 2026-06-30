package evidence

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/managerdoc"
)

const docKindBundle = "evidence_bundle"

var (
	ErrBundleNotFound    = errors.New("evidence bundle not found")
	ErrPermissionDenied  = errors.New("support permission required for evidence bundle")
	ErrInvalidScope      = errors.New("evidence bundle scope is invalid")
	ErrRedactionFailed   = errors.New("evidence bundle redaction failed closed")
	ErrCrossTenantAccess = errors.New("evidence bundle belongs to another tenant")
)

// DefaultRetention is how long a generated bundle is retained.
const DefaultRetention = 14 * 24 * time.Hour

// Collector gathers redaction-candidate sections for a scope. Implementations reuse existing
// diagnostic/evaluation/audit/event records (never raw secrets or unbounded logs).
type Collector interface {
	Collect(ctx context.Context, tenantID string, scope Scope) ([]Section, error)
}

// PermissionGate authorizes support-role bundle generation/access for a tenant.
type PermissionGate interface {
	AllowSupport(ctx context.Context, actor, tenantID string) bool
}

type allowAll struct{}

func (allowAll) AllowSupport(context.Context, string, string) bool { return true }

type emptyCollector struct{}

func (emptyCollector) Collect(context.Context, string, Scope) ([]Section, error) { return nil, nil }

// Manager generates and serves redacted evidence bundles. Bundles + audit are in-memory for this
// slice; bundle content reuses existing records via the collector and is redacted by default.
type Manager struct {
	mu        sync.RWMutex
	env       string
	collector Collector
	perms     PermissionGate
	docs      managerdoc.Store
	bundles   map[string]Bundle
	audit     []AccessEvent
}

// WithStore installs durable persistence for generated evidence bundles and returns the manager.
func (m *Manager) WithStore(s managerdoc.Store) *Manager {
	m.docs = s
	return m
}

// LoadFromStore reloads persisted evidence bundles on startup.
func (m *Manager) LoadFromStore(ctx context.Context) error {
	bundles, err := managerdoc.List[Bundle](ctx, m.docs, docKindBundle)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, b := range bundles {
		m.bundles[b.BundleID] = b
	}
	return nil
}

func NewManager(environmentScope string, collector Collector, perms PermissionGate) *Manager {
	if collector == nil {
		collector = emptyCollector{}
	}
	if perms == nil {
		perms = allowAll{}
	}
	return &Manager{
		env:       strings.TrimSpace(environmentScope),
		collector: collector,
		perms:     perms,
		bundles:   make(map[string]Bundle),
	}
}

// Generate produces a redacted, tenant-scoped, audited evidence bundle. It fails closed when the
// caller lacks support permission or redaction cannot guarantee secret removal.
func (m *Manager) Generate(ctx context.Context, tenantID, actor string, scope Scope) (Bundle, error) {
	if strings.TrimSpace(tenantID) == "" || !validScope(scope) {
		return Bundle{}, ErrInvalidScope
	}
	if !m.perms.AllowSupport(ctx, actor, tenantID) {
		return Bundle{}, ErrPermissionDenied
	}
	collected, err := m.collector.Collect(ctx, tenantID, scope)
	if err != nil {
		return Bundle{}, err
	}
	redacted, ok := redactSections(collected)
	if !ok {
		// Fail closed: do not persist or return a bundle that could leak secrets.
		return Bundle{}, ErrRedactionFailed
	}
	now := time.Now().UTC()
	bundle := Bundle{
		BundleID:           newID("evidence_bundle"),
		TenantID:           strings.TrimSpace(tenantID),
		Actor:              strings.TrimSpace(actor),
		Scope:              scope,
		Sections:           redacted,
		RedactionStatus:    RedactionRedacted,
		CreatedAt:          now,
		RetentionExpiresAt: now.Add(DefaultRetention),
	}
	m.mu.Lock()
	m.bundles[bundle.BundleID] = bundle
	m.audit = append(m.audit, AccessEvent{BundleID: bundle.BundleID, TenantID: bundle.TenantID, Actor: bundle.Actor, Action: "generated", OccurredAt: now})
	m.mu.Unlock()
	_ = managerdoc.Put(ctx, m.docs, docKindBundle, bundle.BundleID, m.env, bundle.TenantID, bundle)
	return bundle, nil
}

// Get returns a bundle for an authorized support actor within the owning tenant, recording an
// access audit event. Cross-tenant access is denied.
func (m *Manager) Get(ctx context.Context, tenantID, actor, bundleID string) (Bundle, error) {
	m.mu.RLock()
	bundle, ok := m.bundles[strings.TrimSpace(bundleID)]
	m.mu.RUnlock()
	if !ok {
		return Bundle{}, ErrBundleNotFound
	}
	if bundle.TenantID != strings.TrimSpace(tenantID) {
		return Bundle{}, ErrCrossTenantAccess
	}
	if !m.perms.AllowSupport(ctx, actor, tenantID) {
		return Bundle{}, ErrPermissionDenied
	}
	m.mu.Lock()
	m.audit = append(m.audit, AccessEvent{BundleID: bundle.BundleID, TenantID: bundle.TenantID, Actor: strings.TrimSpace(actor), Action: "accessed", OccurredAt: time.Now().UTC()})
	m.mu.Unlock()
	return bundle, nil
}

// ListForTenant returns bundle metadata for a tenant (permission-gated).
func (m *Manager) ListForTenant(ctx context.Context, tenantID, actor string) ([]Bundle, error) {
	if !m.perms.AllowSupport(ctx, actor, tenantID) {
		return nil, ErrPermissionDenied
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Bundle, 0)
	for _, b := range m.bundles {
		if b.TenantID == strings.TrimSpace(tenantID) {
			out = append(out, b)
		}
	}
	return out, nil
}

// AuditTrail returns the generation/access audit events for a bundle (FR audit).
func (m *Manager) AuditTrail(bundleID string) []AccessEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]AccessEvent, 0)
	for _, e := range m.audit {
		if e.BundleID == strings.TrimSpace(bundleID) {
			out = append(out, e)
		}
	}
	return out
}

func validScope(scope Scope) bool {
	switch scope.Kind {
	case ScopeRun, ScopeWorkflow, ScopeThread, ScopeConnector, ScopeProvider, ScopeRoutine, ScopeQuotaDenial:
		return strings.TrimSpace(scope.Ref) != ""
	case ScopeTimeWindow:
		return scope.WindowStart != nil && scope.WindowEnd != nil
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
