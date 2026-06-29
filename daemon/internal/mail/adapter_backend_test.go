package mail_test

import (
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterref"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
)

func testResource() integrations.Resource {
	return integrations.Resource{
		IntegrationID:    "int-mail-1",
		DomainKind:       "mail",
		EnvironmentScope: "test",
		ReadinessStatus:  integrations.ReadinessStatusHealthy,
		BackendBinding:   integrations.BackendBinding{BackendKind: integrations.BackendKindAdapterRPC},
	}
}

func TestMailAdapterBackendDispatchesOperations(t *testing.T) {
	client, stop := adapterref.NewPipeClient()
	defer stop()
	backend := mail.NewAdapterBackend(client, 2*time.Second)
	resource := testResource()

	if !backend.SupportsResource(resource) {
		t.Fatal("SupportsResource should be true for mail resource")
	}
	acct, err := backend.ProjectAccount(resource)
	if err != nil {
		t.Fatalf("ProjectAccount: %v", err)
	}
	if _, err := backend.ListThreads(resource, acct, mail.ListThreadsInput{}); err != nil {
		t.Fatalf("ListThreads: %v", err)
	}
	if _, _, err := backend.CreateDraft(resource, acct, mail.CreateDraftInput{}); err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if _, _, err := backend.SendMessage(resource, acct, mail.SendMessageInput{}); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if _, _, _, err := backend.SendDraft(resource, acct, mail.SendDraftInput{}); err != nil {
		t.Fatalf("SendDraft: %v", err)
	}
}

func TestMailRegisterAdapterBackendSelectable(t *testing.T) {
	client, stop := adapterref.NewPipeClient()
	defer stop()
	m := mail.NewManager("test")
	m.RegisterBackend(integrations.BackendKindAdapterRPC, mail.NewAdapterBackend(client, time.Second))
	// Registration is exercised indirectly: a manager operation selecting adapter_rpc must
	// resolve the backend (covered in single-ledger tests). Here we assert no panic/build.
}
