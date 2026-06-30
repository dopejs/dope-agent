package opsreadiness

import "strings"

// MailSmokeInput describes the operator-provided conditions for the real Feishu/Lark mail smoke
// matrix (Roadmap 63 / spec 048). The fake-backend mail suite is always required; the
// real-account rows run only when safe operator credentials are available, otherwise an
// explicit structured skip is recorded.
type MailSmokeInput struct {
	SafeCredentialsAvailable      bool
	Enabled                       bool
	SendReplyForwardExercised     bool   // send/reply/forward live-validation rows ran and classified
	FakeBackendCoveragePassing    bool   // the existing fake mail suite passes
	ContainsRawCredentialMaterial bool   // must stay false; any true is a hard failure
	SkipReason                    string // operator-supplied reason when credentials are unavailable
}

const defaultMailSkipReason = "safe Feishu/Lark mail credentials unavailable in this environment"

// MailRealAccountSmoke builds the mail real-account smoke status. When safe credentials are
// available and send/reply/forward rows ran with no raw credential exposure, it reports pass;
// otherwise it records a structured skip with a reason. It never fabricates a pass without
// exercised send rows.
func MailRealAccountSmoke(in MailSmokeInput) RealAccountSmokeStatus {
	status := RealAccountSmokeStatus{
		Domain:                        "mail",
		SafeCredentialsAvailable:      in.SafeCredentialsAvailable,
		Enabled:                       in.Enabled,
		FakeBackendCoveragePassing:    in.FakeBackendCoveragePassing,
		ContainsRawCredentialMaterial: in.ContainsRawCredentialMaterial,
	}
	switch {
	case in.SafeCredentialsAvailable && in.Enabled && in.SendReplyForwardExercised:
		status.Result = StatusPass
	case in.SafeCredentialsAvailable && in.Enabled:
		status.Result = StatusFail
	default:
		status.Result = StatusSkip
		status.SkipReason = strings.TrimSpace(in.SkipReason)
		if status.SkipReason == "" {
			status.SkipReason = defaultMailSkipReason
		}
	}
	return status
}
