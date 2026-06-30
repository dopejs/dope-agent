package opsreadiness

import (
	"fmt"
	"sort"
	"strings"
)

// RequiredLaunchWorkloads is the non-knowledge parity workload set the public beta launch gate
// MUST exercise (Roadmap 72, FR). Missing required evidence is a no-ship condition.
var RequiredLaunchWorkloads = []string{
	"activation", "setup", "channels", "sessions", "profile_binding", "routines", "webhooks",
	"quota_denial", "diagnostics", "evaluation", "live_validation", "support_bundle",
	"backup", "restore", "upgrade", "rollback",
}

// LaunchGateStatement is the entry-gate rule: context/knowledge/memory work may begin only after
// non-knowledge parity release evidence passes (or residual exceptions are explicitly accepted).
const LaunchGateStatement = "Context, knowledge, and memory work may begin only after non-knowledge parity release evidence passes or residual exceptions are explicitly accepted."

// WorkloadEvidence is one exercised launch-gate workload outcome.
type WorkloadEvidence struct {
	Name   string `json:"name"`             // one of RequiredLaunchWorkloads
	Status string `json:"status"`           // StatusPass | StatusFail | StatusSkip
	Owner  string `json:"owner,omitempty"`  // remediation/classification owner
	Reason string `json:"reason,omitempty"` // required when Status == skip
}

// LaunchGateEvidence is the public beta release evidence index.
type LaunchGateEvidence struct {
	Channels               []RealAccountSmokeStatus `json:"channels"`      // >= 3 channel entries (real or skipped-with-reason)
	ProviderSmoke          []RealAccountSmokeStatus `json:"providerSmoke"` // calendar + mail provider entries
	Workloads              []WorkloadEvidence       `json:"workloads"`
	SoakDurationMet        bool                     `json:"soakDurationMet"`
	SupportBundleValidated bool                     `json:"supportBundleValidated"`
	RedactionValidated     bool                     `json:"redactionValidated"`
}

// LaunchDecision is the ship/no-ship decision plus the entry-gate flag.
type LaunchDecision struct {
	Result                     string   `json:"result"`            // "ship" | "no_ship"
	Reasons                    []string `json:"reasons,omitempty"` // no-ship reasons (empty when ship)
	NonKnowledgeParityComplete bool     `json:"nonKnowledgeParityComplete"`
	GateStatement              string   `json:"gateStatement"`
}

// ValidateLaunchGate evaluates the public beta launch gate and returns a ship/no-ship decision.
// It enforces required-workload coverage, >= 3 channel entries, calendar + mail provider entries,
// soak, support-bundle, and redaction evidence. Skips are accepted only with a reason. Missing
// required evidence is a no-ship condition (FR).
func ValidateLaunchGate(evidence LaunchGateEvidence) LaunchDecision {
	var reasons []string

	// Required workloads: each must be present and not failed; a skip needs an accepted reason.
	byName := make(map[string]WorkloadEvidence, len(evidence.Workloads))
	for _, w := range evidence.Workloads {
		byName[strings.TrimSpace(w.Name)] = w
	}
	for _, required := range RequiredLaunchWorkloads {
		w, ok := byName[required]
		switch {
		case !ok:
			reasons = append(reasons, fmt.Sprintf("missing required workload evidence: %s", required))
		case w.Status == StatusFail:
			reasons = append(reasons, fmt.Sprintf("required workload failed: %s", required))
		case w.Status == StatusSkip && strings.TrimSpace(w.Reason) == "":
			reasons = append(reasons, fmt.Sprintf("skipped workload requires an accepted reason: %s", required))
		case w.Status != StatusPass && w.Status != StatusSkip:
			reasons = append(reasons, fmt.Sprintf("workload has invalid status %q: %s", w.Status, required))
		}
	}

	// At least three channel entries, each valid (real pass or skipped-with-reason, no raw creds).
	if len(evidence.Channels) < 3 {
		reasons = append(reasons, fmt.Sprintf("launch gate requires at least 3 channel entries, got %d", len(evidence.Channels)))
	}
	if err := ValidateRealAccountSmoke(evidence.Channels); err != nil && len(evidence.Channels) > 0 {
		reasons = append(reasons, "channel smoke invalid: "+err.Error())
	}

	// Calendar and mail provider entries must be present and valid.
	providerDomains := map[string]bool{}
	for _, p := range evidence.ProviderSmoke {
		providerDomains[strings.TrimSpace(p.Domain)] = true
	}
	for _, domain := range []string{"calendar", "mail"} {
		if !providerDomains[domain] {
			reasons = append(reasons, fmt.Sprintf("missing %s provider smoke entry", domain))
		}
	}
	if err := ValidateRealAccountSmoke(evidence.ProviderSmoke); err != nil && len(evidence.ProviderSmoke) > 0 {
		reasons = append(reasons, "provider smoke invalid: "+err.Error())
	}

	if !evidence.SoakDurationMet {
		reasons = append(reasons, "full-duration hosted soak not met")
	}
	if !evidence.SupportBundleValidated {
		reasons = append(reasons, "support bundle generation/validation not exercised during soak")
	}
	if !evidence.RedactionValidated {
		reasons = append(reasons, "redaction validation not exercised during soak")
	}

	sort.Strings(reasons)
	decision := LaunchDecision{Result: "ship", NonKnowledgeParityComplete: true, GateStatement: LaunchGateStatement}
	if len(reasons) > 0 {
		decision.Result = "no_ship"
		decision.Reasons = reasons
		decision.NonKnowledgeParityComplete = false
	}
	return decision
}
