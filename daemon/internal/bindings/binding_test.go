package bindings

import (
	"reflect"
	"strings"
	"testing"
)

// B7/B21/FR-020/FR-021/SC-015: the binding domain carries identity only — no filesystem,
// repository, connector handle, or memory/knowledge field. This guards against scope creep
// where workspace binding might silently imply storage or knowledge access.
func TestBindingDomainCarriesIdentityOnly(t *testing.T) {
	forbidden := []string{"filesystem", "filepath", "directory", "secret", "token", "credential", "memory", "knowledge", "embedding", "fshandle"}
	for _, typ := range []reflect.Type{
		reflect.TypeOf(Workspace{}),
		reflect.TypeOf(BindingRule{}),
		reflect.TypeOf(EffectiveBindingSelection{}),
		reflect.TypeOf(CapabilityVisibilityPolicy{}),
		reflect.TypeOf(RuntimeBindingEvidence{}),
	} {
		for i := 0; i < typ.NumField(); i++ {
			name := strings.ToLower(typ.Field(i).Name)
			for _, bad := range forbidden {
				if strings.Contains(name, bad) {
					t.Fatalf("%s.%s implies %q access; binding state must be identity-only", typ.Name(), typ.Field(i).Name, bad)
				}
			}
		}
	}
}

// FR-020: a resolved selection exposes workspace identity but never a filesystem path.
func TestResolvedSelectionExposesIdentityNotAccess(t *testing.T) {
	sel := ResolveSelection(ResolutionInput{
		TenantDefaultProfileID:   "prof_1",
		TenantDefaultWorkspaceID: "ws_1",
		ProfileAvailable:         func(string) bool { return true },
		WorkspaceAvailable:       func(string) bool { return true },
	})
	if sel.SelectedWorkspaceID != "ws_1" {
		t.Fatalf("expected workspace identity, got %q", sel.SelectedWorkspaceID)
	}
	if strings.Contains(sel.SelectedWorkspaceID, "/") {
		t.Fatalf("workspace identity must not look like a filesystem path")
	}
}
