package execprofile

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/managerdoc"
)

const (
	docKindExecProfile   = "exec_profile"
	docKindExecSelection = "exec_selection"
)

var (
	ErrProfileNotFound    = errors.New("execution profile not found")
	ErrInvalidProfile     = errors.New("execution profile definition is invalid")
	ErrProfileUnavailable = errors.New("execution profile is unavailable")
	ErrPermissionDenied   = errors.New("profile selection is not permitted")
)

// HealthChecker reports the live backend health of a profile (injectable; fake in tests).
type HealthChecker interface {
	Health(ctx context.Context, profile ExecutionProfile) (HealthStatus, string)
}

// RequirementChecker reports which of a profile's environment requirements are unmet.
type RequirementChecker interface {
	Unmet(ctx context.Context, requirements []string) []string
}

// PermissionGate gates profile selection (FR-004 permission-gated + auditable).
type PermissionGate interface {
	Allow(ctx context.Context, tenantID, profileID string) bool
}

type readyHealth struct{}

func (readyHealth) Health(context.Context, ExecutionProfile) (HealthStatus, string) {
	return HealthReady, ""
}

type allMet struct{}

func (allMet) Unmet(context.Context, []string) []string { return nil }

type allowAll struct{}

func (allowAll) Allow(context.Context, string, string) bool { return true }

// Manager projects execution profiles and selections. Profiles + selections are in-memory with
// Restore; the sandbox/policy layer remains authoritative for actual execution permission.
type Manager struct {
	mu         sync.RWMutex
	env        string
	health     HealthChecker
	reqs       RequirementChecker
	perms      PermissionGate
	docs       managerdoc.Store
	profiles   map[string]ExecutionProfile
	selections map[string]Selection // tenantID -> selection
}

// WithStore installs durable persistence for profiles + selections and returns the manager.
func (m *Manager) WithStore(s managerdoc.Store) *Manager {
	m.docs = s
	return m
}

// LoadFromStore reloads persisted profiles + selections on startup.
func (m *Manager) LoadFromStore(ctx context.Context) error {
	profiles, err := managerdoc.List[ExecutionProfile](ctx, m.docs, docKindExecProfile)
	if err != nil {
		return err
	}
	selections, err := managerdoc.List[Selection](ctx, m.docs, docKindExecSelection)
	if err != nil {
		return err
	}
	m.Restore(profiles, selections)
	return nil
}

func NewManager(environmentScope string, health HealthChecker, reqs RequirementChecker, perms PermissionGate) *Manager {
	if health == nil {
		health = readyHealth{}
	}
	if reqs == nil {
		reqs = allMet{}
	}
	if perms == nil {
		perms = allowAll{}
	}
	return &Manager{
		env:        strings.TrimSpace(environmentScope),
		health:     health,
		reqs:       reqs,
		perms:      perms,
		profiles:   make(map[string]ExecutionProfile),
		selections: make(map[string]Selection),
	}
}

// RegisterProfile inserts or replaces an execution profile.
func (m *Manager) RegisterProfile(profile ExecutionProfile) (ExecutionProfile, error) {
	if strings.TrimSpace(profile.Name) == "" || !validBackend(profile.BackendKind) {
		return ExecutionProfile{}, ErrInvalidProfile
	}
	if profile.RiskTier == "" {
		profile.RiskTier = RiskMedium
	}
	if strings.TrimSpace(profile.ProfileID) == "" {
		profile.ProfileID = newID("exec_profile")
	}
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = time.Now().UTC()
	}
	m.mu.Lock()
	m.profiles[profile.ProfileID] = profile
	m.mu.Unlock()
	_ = managerdoc.Put(context.Background(), m.docs, docKindExecProfile, profile.ProfileID, m.env, "", profile)
	return profile, nil
}

func (m *Manager) status(ctx context.Context, profile ExecutionProfile) ProfileStatus {
	health, reason := m.health.Health(ctx, profile)
	unmet := m.reqs.Unmet(ctx, profile.Requirements)
	available := health == HealthReady && len(unmet) == 0
	if !available && reason == "" {
		if health != HealthReady {
			reason = "backend " + string(health)
		} else if len(unmet) > 0 {
			reason = "unmet requirements: " + strings.Join(unmet, ", ")
		}
	}
	return ProfileStatus{ProfileID: profile.ProfileID, Health: health, Reason: reason, UnmetRequirements: unmet, Available: available}
}

// ListProfiles returns all profiles with live status (FR-001).
func (m *Manager) ListProfiles(ctx context.Context) []ProfileProjection {
	m.mu.RLock()
	profiles := make([]ExecutionProfile, 0, len(m.profiles))
	for _, p := range m.profiles {
		profiles = append(profiles, p)
	}
	m.mu.RUnlock()
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ProfileID < profiles[j].ProfileID })
	out := make([]ProfileProjection, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, ProfileProjection{Profile: p, Status: m.status(ctx, p)})
	}
	return out
}

func (m *Manager) GetProfile(ctx context.Context, profileID string) (ProfileProjection, error) {
	m.mu.RLock()
	profile, ok := m.profiles[strings.TrimSpace(profileID)]
	m.mu.RUnlock()
	if !ok {
		return ProfileProjection{}, ErrProfileNotFound
	}
	return ProfileProjection{Profile: profile, Status: m.status(ctx, profile)}, nil
}

// ExplainDenial explains which profiles can run a tool requiring the given capabilities, and why
// the others cannot (missing capabilities or unavailability).
func (m *Manager) ExplainDenial(ctx context.Context, required []string) DenialExplanation {
	exp := DenialExplanation{
		RequiredCapabilities: required,
		EligibleProfiles:     []string{},
		MissingCapabilities:  map[string][]string{},
		Unavailable:          map[string]string{},
	}
	for _, proj := range m.ListProfiles(ctx) {
		missing := missingCapabilities(proj.Profile.Provides, required)
		if len(missing) > 0 {
			exp.MissingCapabilities[proj.Profile.ProfileID] = missing
			continue
		}
		if !proj.Status.Available {
			exp.Unavailable[proj.Profile.ProfileID] = firstNonEmpty(proj.Status.Reason, "unavailable")
			continue
		}
		exp.EligibleProfiles = append(exp.EligibleProfiles, proj.Profile.ProfileID)
	}
	return exp
}

// CompatibilityFor reports profiles compatible/incompatible with a catalog item's capability
// requirements (FR-003), regardless of live availability.
func (m *Manager) CompatibilityFor(ctx context.Context, required []string) Compatibility {
	out := Compatibility{RequiredCapabilities: required, Compatible: []string{}, Incompatible: []string{}}
	for _, proj := range m.ListProfiles(ctx) {
		if len(missingCapabilities(proj.Profile.Provides, required)) == 0 {
			out.Compatible = append(out.Compatible, proj.Profile.ProfileID)
		} else {
			out.Incompatible = append(out.Incompatible, proj.Profile.ProfileID)
		}
	}
	return out
}

// SelectProfile sets a tenant's execution profile, permission-gated and audited (FR-004). It
// fails closed when the profile is unavailable.
func (m *Manager) SelectProfile(ctx context.Context, tenantID, profileID, actor string) (Selection, error) {
	proj, err := m.GetProfile(ctx, profileID)
	if err != nil {
		return Selection{}, err
	}
	if !m.perms.Allow(ctx, tenantID, profileID) {
		return Selection{}, ErrPermissionDenied
	}
	if !proj.Status.Available {
		return Selection{}, ErrProfileUnavailable
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	selection := m.selections[strings.TrimSpace(tenantID)]
	now := time.Now().UTC()
	selection.TenantID = strings.TrimSpace(tenantID)
	selection.ProfileID = profileID
	selection.History = append(selection.History, SelectionEvent{ProfileID: profileID, Actor: actor, OccurredAt: now})
	selection.UpdatedAt = now
	m.selections[selection.TenantID] = selection
	_ = managerdoc.Put(context.Background(), m.docs, docKindExecSelection, selection.TenantID, m.env, selection.TenantID, selection)
	return selection, nil
}

func (m *Manager) SelectionForTenant(tenantID string) (Selection, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.selections[strings.TrimSpace(tenantID)]
	return s, ok
}

// Restore reloads persisted profiles + selections.
func (m *Manager) Restore(profiles []ExecutionProfile, selections []Selection) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.profiles = make(map[string]ExecutionProfile, len(profiles))
	for _, p := range profiles {
		m.profiles[p.ProfileID] = p
	}
	m.selections = make(map[string]Selection, len(selections))
	for _, s := range selections {
		m.selections[s.TenantID] = s
	}
}

// missingCapabilities returns the required capabilities a profile does not provide.
func missingCapabilities(provides, required []string) []string {
	have := make(map[string]bool, len(provides))
	for _, p := range provides {
		have[strings.ToLower(strings.TrimSpace(p))] = true
	}
	var missing []string
	for _, r := range required {
		if !have[strings.ToLower(strings.TrimSpace(r))] {
			missing = append(missing, r)
		}
	}
	return missing
}

func validBackend(k BackendKind) bool {
	switch k {
	case BackendSubprocess, BackendDocker, BackendSSH, BackendLocalShell:
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_fallback"
	}
	return prefix + "_" + hex.EncodeToString(buf)
}
