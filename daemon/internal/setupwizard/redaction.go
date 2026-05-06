package setupwizard

import (
	"encoding/json"
	"strings"
)

var forbiddenEvidenceFieldNames = []string{
	"value",
	"authorizationCode",
	"accessToken",
	"refreshToken",
	"providerToken",
	"callbackPayload",
	"Authorization",
	"clientSecret",
	"providerSecret",
}

func RedactedSecretEvidence(secretRef, displayName string) map[string]string {
	out := map[string]string{
		"redactionRule": "secret_metadata_only",
		"secretRef":     strings.TrimSpace(secretRef),
	}
	if trimmed := strings.TrimSpace(displayName); trimmed != "" {
		out["displayName"] = trimmed
	}
	return out
}

func RedactedOAuthEvidence(result OAuthResult, accountLabel string) map[string]string {
	out := map[string]string{
		"redactionRule":       "oauth_metadata_only",
		"authorizationStatus": string(result),
	}
	if trimmed := strings.TrimSpace(accountLabel); trimmed != "" {
		out["accountLabel"] = trimmed
	}
	return out
}

func ContainsForbiddenEvidence(value any, forbiddenValues []string) bool {
	raw, err := json.Marshal(value)
	if err != nil {
		return true
	}
	text := string(raw)
	for _, field := range forbiddenEvidenceFieldNames {
		if strings.Contains(text, `"`+field+`"`) {
			return true
		}
	}
	for _, value := range forbiddenValues {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" && strings.Contains(text, trimmed) {
			return true
		}
	}
	return false
}

func failClosed(session SetupSession, reason string) SetupSession {
	session.State = StateActionRequired
	session.SafeUseMode = SafeUseBlocked
	session.ReasonCode = firstNonEmpty(reason, ReasonRedactionFailedClosed)
	session.Retryable = false
	session.RemediationOwner = OwnerOperator
	session.RedactionStatus = RedactionFailedClosed
	session.DiagnosticResultID = ensureDiagnosticID(session)
	return session
}
