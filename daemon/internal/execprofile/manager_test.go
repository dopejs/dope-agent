package execprofile

import (
	"context"
	"errors"
	"testing"
)

type fixedHealth map[string]HealthStatus

func (f fixedHealth) Health(_ context.Context, p ExecutionProfile) (HealthStatus, string) {
	if s, ok := f[p.ProfileID]; ok {
		return s, ""
	}
	return HealthReady, ""
}

type denyPerms struct{}

func (denyPerms) Allow(context.Context, string, string) bool { return false }

func seed(t *testing.T, m *Manager) (subproc, docker, ssh ExecutionProfile) {
	t.Helper()
	var err error
	subproc, err = m.RegisterProfile(ExecutionProfile{ProfileID: "p_subproc", Name: "Subprocess", BackendKind: BackendSubprocess, RiskTier: RiskLow, Provides: []string{"local_fs"}})
	if err != nil {
		t.Fatalf("register subproc: %v", err)
	}
	docker, _ = m.RegisterProfile(ExecutionProfile{ProfileID: "p_docker", Name: "Docker", BackendKind: BackendDocker, RiskTier: RiskMedium, Provides: []string{"local_fs", "docker", "network"}, Requirements: []string{"docker_daemon"}})
	ssh, _ = m.RegisterProfile(ExecutionProfile{ProfileID: "p_ssh", Name: "SSH", BackendKind: BackendSSH, RiskTier: RiskHigh, Provides: []string{"network", "remote_shell"}, Requirements: []string{"ssh_credentials"}})
	return
}

// FR-001: profiles list with live status, requirements, and risk.
func TestListProfilesStatus(t *testing.T) {
	m := NewManager("test", fixedHealth{"p_ssh": HealthUnavailable}, nil, nil)
	seed(t, m)
	projections := m.ListProfiles(context.Background())
	if len(projections) != 3 {
		t.Fatalf("want 3 profiles, got %d", len(projections))
	}
	byID := map[string]ProfileProjection{}
	for _, p := range projections {
		byID[p.Profile.ProfileID] = p
	}
	if !byID["p_subproc"].Status.Available {
		t.Fatalf("subprocess should be available: %+v", byID["p_subproc"].Status)
	}
	if byID["p_ssh"].Status.Available || byID["p_ssh"].Status.Health != HealthUnavailable {
		t.Fatalf("ssh should be unavailable: %+v", byID["p_ssh"].Status)
	}
}

// FR-002: a tool needing docker is denied on profiles lacking it, with missing-capability detail.
func TestExplainDenial(t *testing.T) {
	m := NewManager("test", nil, nil, nil)
	seed(t, m)
	exp := m.ExplainDenial(context.Background(), []string{"docker"})
	if len(exp.EligibleProfiles) != 1 || exp.EligibleProfiles[0] != "p_docker" {
		t.Fatalf("only docker profile should be eligible: %+v", exp.EligibleProfiles)
	}
	if miss := exp.MissingCapabilities["p_subproc"]; len(miss) != 1 || miss[0] != "docker" {
		t.Fatalf("subprocess should be missing docker: %+v", exp.MissingCapabilities)
	}
}

// FR-002: requirement unavailability is surfaced distinctly from missing capabilities.
func TestExplainDenialUnavailable(t *testing.T) {
	// docker daemon requirement unmet -> docker profile unavailable (not missing-capability).
	m := NewManager("test", nil, unmetReqs{"docker_daemon"}, nil)
	seed(t, m)
	exp := m.ExplainDenial(context.Background(), []string{"docker"})
	if len(exp.EligibleProfiles) != 0 {
		t.Fatalf("no profile should be eligible when docker daemon is missing: %+v", exp.EligibleProfiles)
	}
	if _, ok := exp.Unavailable["p_docker"]; !ok {
		t.Fatalf("docker profile should be unavailable with a reason: %+v", exp.Unavailable)
	}
}

// FR-003: catalog-item capability requirements map to compatible/incompatible profiles.
func TestCompatibility(t *testing.T) {
	m := NewManager("test", nil, nil, nil)
	seed(t, m)
	compat := m.CompatibilityFor(context.Background(), []string{"network"})
	if len(compat.Compatible) != 2 { // docker + ssh provide network
		t.Fatalf("two profiles should provide network: %+v", compat)
	}
}

// FR-004: selection is permission-gated, audited, and fails closed when unavailable.
func TestSelectProfileGating(t *testing.T) {
	m := NewManager("test", fixedHealth{"p_ssh": HealthUnavailable}, nil, nil)
	seed(t, m)
	sel, err := m.SelectProfile(context.Background(), "ten_a", "p_subproc", "op")
	if err != nil || sel.ProfileID != "p_subproc" || len(sel.History) != 1 {
		t.Fatalf("select should succeed + audit: sel=%+v err=%v", sel, err)
	}
	if _, err := m.SelectProfile(context.Background(), "ten_a", "p_ssh", "op"); !errors.Is(err, ErrProfileUnavailable) {
		t.Fatalf("unavailable profile selection must fail closed: %v", err)
	}

	denyM := NewManager("test", nil, nil, denyPerms{})
	seed(t, denyM)
	if _, err := denyM.SelectProfile(context.Background(), "ten_a", "p_subproc", "op"); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("denied permission must block selection: %v", err)
	}
}

type unmetReqs []string

func (u unmetReqs) Unmet(_ context.Context, requirements []string) []string {
	want := map[string]bool{}
	for _, r := range u {
		want[r] = true
	}
	var out []string
	for _, r := range requirements {
		if want[r] {
			out = append(out, r)
		}
	}
	return out
}
