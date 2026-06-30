package evidence

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeCollector struct{ sections []Section }

func (f fakeCollector) Collect(context.Context, string, Scope) ([]Section, error) {
	return f.sections, nil
}

type denySupport struct{}

func (denySupport) AllowSupport(context.Context, string, string) bool { return false }

func routineScope() Scope { return Scope{Kind: ScopeRoutine, Ref: "routine_1"} }

// FR + US1: generate a redacted, audited bundle of summaries/links for a routine failure.
func TestGenerateRedactsAndAudits(t *testing.T) {
	collector := fakeCollector{sections: []Section{{
		Kind:         "routine",
		ResourceRefs: []string{"routine_1", "sched_1"},
		Summary:      map[string]string{"state": "failed", "accessToken": "sk-secret-abc123", "lastError": "provider_unavailable"},
		Links:        []string{"/v1/routines/routine_1"},
	}}}
	m := NewManager("test", collector, nil)
	bundle, err := m.Generate(context.Background(), "ten_a", "support@dope", routineScope())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if bundle.RedactionStatus != RedactionRedacted {
		t.Fatalf("bundle should be redacted: %+v", bundle)
	}
	// Sensitive key redacted; non-sensitive summary preserved.
	if got := bundle.Sections[0].Summary["accessToken"]; got != redactedPlaceholder {
		t.Fatalf("accessToken not redacted: %q", got)
	}
	if bundle.Sections[0].Summary["lastError"] != "provider_unavailable" {
		t.Fatalf("non-sensitive summary should be preserved")
	}
	// No raw secret material anywhere in the bundle.
	for _, s := range bundle.Sections {
		for _, v := range s.Summary {
			if strings.Contains(v, "sk-secret") {
				t.Fatalf("raw secret leaked into bundle: %q", v)
			}
		}
	}
	if audit := m.AuditTrail(bundle.BundleID); len(audit) != 1 || audit[0].Action != "generated" {
		t.Fatalf("generation not audited: %+v", audit)
	}
}

// FR: redaction fails closed when a non-sensitive-keyed value carries raw secret material.
func TestRedactionFailsClosed(t *testing.T) {
	collector := fakeCollector{sections: []Section{{
		Kind:    "connector",
		Summary: map[string]string{"note": "set header Authorization: Bearer abcdef0123456789 to retry"},
	}}}
	m := NewManager("test", collector, nil)
	if _, err := m.Generate(context.Background(), "ten_a", "support@dope", Scope{Kind: ScopeConnector, Ref: "c1"}); !errors.Is(err, ErrRedactionFailed) {
		t.Fatalf("expected redaction fail-closed, got %v", err)
	}
}

// FR: generation/access is permission-gated; cross-tenant access is denied.
func TestPermissionAndTenantIsolation(t *testing.T) {
	m := NewManager("test", fakeCollector{}, denySupport{})
	if _, err := m.Generate(context.Background(), "ten_a", "user@dope", routineScope()); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("non-support generation must be denied: %v", err)
	}

	open := NewManager("test", fakeCollector{}, nil)
	bundle, err := open.Generate(context.Background(), "ten_a", "support@dope", routineScope())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if _, err := open.Get(context.Background(), "ten_b", "support@dope", bundle.BundleID); !errors.Is(err, ErrCrossTenantAccess) {
		t.Fatalf("cross-tenant access must be denied: %v", err)
	}
	if _, err := open.Get(context.Background(), "ten_a", "support@dope", bundle.BundleID); err != nil {
		t.Fatalf("owning-tenant access should succeed: %v", err)
	}
	// Access is audited (generated + accessed).
	if audit := open.AuditTrail(bundle.BundleID); len(audit) != 2 {
		t.Fatalf("access not audited: %+v", audit)
	}
}

// FR: an invalid scope is rejected before collection.
func TestInvalidScope(t *testing.T) {
	m := NewManager("test", fakeCollector{}, nil)
	if _, err := m.Generate(context.Background(), "ten_a", "support@dope", Scope{Kind: ScopeRoutine}); !errors.Is(err, ErrInvalidScope) {
		t.Fatalf("scope without ref must be invalid: %v", err)
	}
}
