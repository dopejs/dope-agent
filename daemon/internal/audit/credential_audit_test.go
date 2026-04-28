package audit_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/secrets"
)

const r37AuditLeakSentinel = "R37_FAKE_SECRET_TENANT_A_DO_NOT_LEAK"

func TestCredentialAuditEventFieldsAndGranularity(t *testing.T) {
	createdAt := time.Date(2026, 4, 27, 10, 0, 0, 0, time.UTC)
	event := audit.BuildCredentialAuditEvent(audit.CredentialAuditInput{
		TenantID:        "ten_r37_a",
		PrincipalID:     "prn_r37_a",
		ResourceKind:    secrets.ResourceKindMCPTool,
		ResourceID:      "filesystem/search",
		Action:          secrets.AuditActionSecretUse,
		Outcome:         identity.AuditOutcomeSucceeded,
		ReasonCode:      "credential_used",
		SecretRef:       "mcp/filesystem/token",
		SecretVersionID: "secver_r37_a",
		SecretRefs:      []string{"mcp/filesystem/token", "mcp/filesystem/refresh"},
		CreatedAt:       createdAt,
	})

	if event.EventKind != audit.CredentialEventKind {
		t.Fatalf("EventKind=%q, want %q", event.EventKind, audit.CredentialEventKind)
	}
	if event.TenantID != "ten_r37_a" || event.PrincipalID != "prn_r37_a" {
		t.Fatalf("tenant/principal not preserved: %#v", event)
	}
	if event.Outcome != identity.AuditOutcomeSucceeded || event.ReasonCode != "credential_used" {
		t.Fatalf("outcome/reason not preserved: %#v", event)
	}
	if got := event.Document["secretRefCount"]; got != 2 {
		t.Fatalf("secretRefCount=%v, want 2", got)
	}
}

func TestCredentialAuditFailsClosedWithoutSecretMaterial(t *testing.T) {
	event := audit.BuildCredentialAuditEvent(audit.CredentialAuditInput{
		TenantID:     "ten_r37_a",
		PrincipalID:  "prn_r37_a",
		ResourceKind: secrets.ResourceKindTenantSecret,
		ResourceID:   "sec_r37_a",
		Action:       secrets.AuditActionCredentialDenied,
		Outcome:      identity.AuditOutcomeFailedClosed,
		ReasonCode:   "credential_store_unavailable",
		SecretRefs:   []string{"tenant-secret/ref"},
	})

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	if strings.Contains(string(data), r37AuditLeakSentinel) {
		t.Fatalf("credential audit leaked sentinel: %s", string(data))
	}
	if !strings.Contains(string(data), "secret_ref_only") {
		t.Fatalf("credential audit did not include redaction rule: %s", string(data))
	}
}

func TestCredentialUseAuditGranularityByRuntimeSurface(t *testing.T) {
	cases := []struct {
		name         string
		resourceKind secrets.ResourceKind
		resourceID   string
		reasonCode   string
	}{
		{name: "run", resourceKind: secrets.ResourceKind("run"), resourceID: "run_r37_a", reasonCode: "credential_bearing_run"},
		{name: "connector", resourceKind: secrets.ResourceKindConnector, resourceID: "discord-main", reasonCode: "connector_ingress_accepted"},
		{name: "mcp", resourceKind: secrets.ResourceKindMCPTool, resourceID: "filesystem/search", reasonCode: "mcp_tool_invoked"},
		{name: "sandbox", resourceKind: secrets.ResourceKindSandboxPolicy, resourceID: "sandbox_scope", reasonCode: "sandbox_secret_scope_prepared"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			event := audit.BuildCredentialAuditEvent(audit.CredentialAuditInput{
				TenantID:     "ten_r37_a",
				PrincipalID:  "prn_r37_a",
				ResourceKind: tc.resourceKind,
				ResourceID:   tc.resourceID,
				Action:       secrets.AuditActionSecretUse,
				Outcome:      identity.AuditOutcomeSucceeded,
				ReasonCode:   tc.reasonCode,
				SecretRefs:   []string{"shared/runtime-token", "shared/refresh-token"},
			})
			if event.Document["resourceKind"] != string(tc.resourceKind) || event.Document["resourceId"] != tc.resourceID {
				t.Fatalf("audit did not preserve resource granularity: %+v", event)
			}
			data, err := json.Marshal(event)
			if err != nil {
				t.Fatalf("marshal event: %v", err)
			}
			if strings.Contains(string(data), r37AuditLeakSentinel) {
				t.Fatalf("credential use audit leaked sentinel: %s", string(data))
			}
		})
	}
}
