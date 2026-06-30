package opsreadiness

import "testing"

func channelSkip(domain string) RealAccountSmokeStatus {
	return RealAccountSmokeStatus{Domain: domain, SafeCredentialsAvailable: false, SkipReason: "no safe " + domain + " creds in CI", FakeBackendCoveragePassing: true}
}

func passingWorkloads() []WorkloadEvidence {
	out := make([]WorkloadEvidence, 0, len(RequiredLaunchWorkloads))
	for _, name := range RequiredLaunchWorkloads {
		out = append(out, WorkloadEvidence{Name: name, Status: StatusPass, Owner: "release"})
	}
	return out
}

func passingEvidence() LaunchGateEvidence {
	return LaunchGateEvidence{
		Channels:               []RealAccountSmokeStatus{channelSkip("discord"), channelSkip("slack"), channelSkip("telegram")},
		ProviderSmoke:          []RealAccountSmokeStatus{CalendarRealAccountSmoke(CalendarSmokeInput{FakeBackendCoveragePassing: true}), MailRealAccountSmoke(MailSmokeInput{FakeBackendCoveragePassing: true})},
		Workloads:              passingWorkloads(),
		SoakDurationMet:        true,
		SupportBundleValidated: true,
		RedactionValidated:     true,
	}
}

// FR + US1: a complete evidence index with reasoned skips ships and marks non-knowledge parity.
func TestLaunchGateShips(t *testing.T) {
	decision := ValidateLaunchGate(passingEvidence())
	if decision.Result != "ship" || !decision.NonKnowledgeParityComplete {
		t.Fatalf("expected ship, got %+v", decision)
	}
	if decision.GateStatement == "" {
		t.Fatal("ship decision should carry the entry-gate statement")
	}
}

// FR: missing required workload evidence is a no-ship condition.
func TestLaunchGateMissingWorkloadNoShip(t *testing.T) {
	ev := passingEvidence()
	ev.Workloads = ev.Workloads[:len(ev.Workloads)-1] // drop "rollback"
	decision := ValidateLaunchGate(ev)
	if decision.Result != "no_ship" || decision.NonKnowledgeParityComplete {
		t.Fatalf("missing workload must be no-ship: %+v", decision)
	}
	if !containsSubstr(decision.Reasons, "rollback") {
		t.Fatalf("no-ship reason should name the missing workload: %+v", decision.Reasons)
	}
}

// FR: a failed required workload is a no-ship condition.
func TestLaunchGateFailedWorkloadNoShip(t *testing.T) {
	ev := passingEvidence()
	ev.Workloads[0].Status = StatusFail
	if d := ValidateLaunchGate(ev); d.Result != "no_ship" {
		t.Fatalf("failed workload must be no-ship: %+v", d)
	}
}

// FR: at least three channel entries are required.
func TestLaunchGateRequiresThreeChannels(t *testing.T) {
	ev := passingEvidence()
	ev.Channels = ev.Channels[:2]
	if d := ValidateLaunchGate(ev); d.Result != "no_ship" {
		t.Fatalf("fewer than 3 channels must be no-ship: %+v", d)
	}
}

// FR: calendar and mail provider entries are required.
func TestLaunchGateRequiresProviderEntries(t *testing.T) {
	ev := passingEvidence()
	ev.ProviderSmoke = []RealAccountSmokeStatus{CalendarRealAccountSmoke(CalendarSmokeInput{FakeBackendCoveragePassing: true})} // mail missing
	d := ValidateLaunchGate(ev)
	if d.Result != "no_ship" || !containsSubstr(d.Reasons, "mail provider") {
		t.Fatalf("missing mail provider must be no-ship: %+v", d)
	}
}

// FR: soak / support-bundle / redaction evidence are required.
func TestLaunchGateRequiresSoakSupportRedaction(t *testing.T) {
	for _, mutate := range []func(*LaunchGateEvidence){
		func(e *LaunchGateEvidence) { e.SoakDurationMet = false },
		func(e *LaunchGateEvidence) { e.SupportBundleValidated = false },
		func(e *LaunchGateEvidence) { e.RedactionValidated = false },
	} {
		ev := passingEvidence()
		mutate(&ev)
		if d := ValidateLaunchGate(ev); d.Result != "no_ship" {
			t.Fatalf("missing soak/support/redaction evidence must be no-ship: %+v", d)
		}
	}
}

// FR: a skipped workload requires an accepted reason.
func TestLaunchGateSkipNeedsReason(t *testing.T) {
	ev := passingEvidence()
	ev.Workloads[0].Status = StatusSkip
	ev.Workloads[0].Reason = ""
	if d := ValidateLaunchGate(ev); d.Result != "no_ship" {
		t.Fatalf("skip without reason must be no-ship: %+v", d)
	}
	ev.Workloads[0].Reason = "not applicable in CI; accepted by release owner"
	if d := ValidateLaunchGate(ev); d.Result != "ship" {
		t.Fatalf("skip with accepted reason should ship: %+v", d)
	}
}

func containsSubstr(items []string, substr string) bool {
	for _, s := range items {
		if len(substr) > 0 && len(s) >= len(substr) && indexOf(s, substr) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
