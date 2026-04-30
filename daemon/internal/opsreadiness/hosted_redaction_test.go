package opsreadiness

import "testing"

func TestHostedRedactionRejectsSecretsInEvidencePayloads(t *testing.T) {
	assertValid(t, ValidateHostedRedaction("manifest", sampleHostedManifest()))
	assertInvalidContains(t, ValidateHostedRedaction("logs", map[string]string{
		"line": "Authorization: Bearer access_token",
	}), "credential")
}
