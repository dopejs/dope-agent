package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/setupwizard"
)

func TestSetupWizardAPIRecoveryRoutesAndInspectionDenials(t *testing.T) {
	service := setupwizard.NewService(setupwizard.ServiceDependencies{Store: setupwizard.NewMemoryStore()})
	actor := setupWizardAPITenantContext("ten_setup_recovery", identity.PermissionSecretsManage, identity.PermissionIntegrationsManage, identity.PermissionCredentialsInspect)

	startRec := exerciseSetupWizardRoute(service, actor, http.MethodPost, "/v1/setup/sessions", `{"targetId":"provider.openai_compatible","setupStyle":"submitted_secret","source":"wizard"}`)
	if startRec.Code != http.StatusCreated {
		t.Fatalf("start status=%d body=%s", startRec.Code, startRec.Body.String())
	}
	var body struct {
		Session setupwizard.SetupSession `json:"session"`
	}
	if err := json.Unmarshal(startRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode start: %v", err)
	}

	for _, tc := range []struct {
		action string
		state  setupwizard.SetupState
	}{
		{"cancel", setupwizard.StateCancelled},
		{"retry", setupwizard.StateInProgress},
		{"replace", setupwizard.StateInProgress},
		{"disable", setupwizard.StateDisabled},
	} {
		rec := exerciseSetupWizardRoute(service, actor, http.MethodPost, "/v1/setup/sessions/"+body.Session.SetupSessionID+"/"+tc.action, "{}")
		if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"state":"`+tc.state+`"`)) {
			t.Fatalf("%s status=%d body=%s", tc.action, rec.Code, rec.Body.String())
		}
	}

	diagnosticsRec := exerciseSetupWizardRoute(service, actor, http.MethodGet, "/v1/setup/sessions/"+body.Session.SetupSessionID+"/diagnostics", "")
	if diagnosticsRec.Code != http.StatusOK || !bytes.Contains(diagnosticsRec.Body.Bytes(), []byte(`"redactionStatus":"redacted"`)) {
		t.Fatalf("diagnostics status=%d body=%s", diagnosticsRec.Code, diagnosticsRec.Body.String())
	}

	noInspect := setupWizardAPITenantContext("ten_setup_recovery", identity.PermissionSecretsManage, identity.PermissionIntegrationsManage)
	for _, path := range []string{"/v1/setup/sessions", "/v1/setup/sessions/" + body.Session.SetupSessionID, "/v1/setup/sessions/" + body.Session.SetupSessionID + "/diagnostics"} {
		rec := exerciseSetupWizardRoute(service, noInspect, http.MethodGet, path, "")
		if rec.Code != http.StatusForbidden || bytes.Contains(rec.Body.Bytes(), []byte(body.Session.SetupSessionID)) {
			t.Fatalf("expected inspection denial without setup disclosure for %s, got %d body=%s", path, rec.Code, rec.Body.String())
		}
	}
}
