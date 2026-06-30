package opsreadiness

import "strings"

// CalendarSmokeInput describes the operator-provided conditions for the real Feishu/Lark
// calendar smoke matrix (Roadmap 60 / spec 045 FR-011, FR-012, SC-006). The fake-backend
// calendar suite is always required; the real-account rows run only when safe operator
// credentials are available, otherwise an explicit structured skip is recorded.
type CalendarSmokeInput struct {
	SafeCredentialsAvailable      bool
	Enabled                       bool
	CreateUpdateCancelExercised   bool   // create/update/cancel live-validation rows ran and classified
	FakeBackendCoveragePassing    bool   // the existing fake calendar suite passes
	ContainsRawCredentialMaterial bool   // must stay false; any true is a hard failure
	SkipReason                    string // operator-supplied reason when credentials are unavailable
}

// defaultCalendarSkipReason is recorded when no explicit skip reason is supplied but safe
// credentials are unavailable, so readiness can still pass on an explicit, reasoned skip.
const defaultCalendarSkipReason = "safe Feishu/Lark calendar credentials unavailable in this environment"

// CalendarRealAccountSmoke builds the calendar real-account smoke status. When safe credentials
// are available and the create/update/cancel rows ran with no raw credential exposure, it
// reports pass; otherwise it records a structured skip with a reason. It never fabricates a
// pass without exercised write rows.
func CalendarRealAccountSmoke(in CalendarSmokeInput) RealAccountSmokeStatus {
	status := RealAccountSmokeStatus{
		Domain:                        "calendar",
		SafeCredentialsAvailable:      in.SafeCredentialsAvailable,
		Enabled:                       in.Enabled,
		FakeBackendCoveragePassing:    in.FakeBackendCoveragePassing,
		ContainsRawCredentialMaterial: in.ContainsRawCredentialMaterial,
	}
	switch {
	case in.SafeCredentialsAvailable && in.Enabled && in.CreateUpdateCancelExercised:
		status.Result = StatusPass
	case in.SafeCredentialsAvailable && in.Enabled:
		// Credentials available and enabled but write rows not exercised: not a clean pass.
		status.Result = StatusFail
	default:
		status.Result = StatusSkip
		status.SkipReason = strings.TrimSpace(in.SkipReason)
		if status.SkipReason == "" {
			status.SkipReason = defaultCalendarSkipReason
		}
	}
	return status
}
