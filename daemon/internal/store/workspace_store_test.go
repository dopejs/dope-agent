package store

import (
	"context"
	"sync"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/bindings"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

func newBindingTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// B6/FR-025: the default workspace is provisioned lazily and exactly once per tenant.
func TestEnsureDefaultWorkspace_LazyAndIdempotent(t *testing.T) {
	s := newBindingTestStore(t)
	ctx := context.Background()
	ws1, err := s.EnsureDefaultWorkspace(ctx, "ten_a")
	if err != nil {
		t.Fatalf("EnsureDefaultWorkspace: %v", err)
	}
	if !ws1.IsDefault || ws1.Status != bindings.WorkspaceActive {
		t.Fatalf("unexpected default workspace: %+v", ws1)
	}
	ws2, err := s.EnsureDefaultWorkspace(ctx, "ten_a")
	if err != nil {
		t.Fatalf("second EnsureDefaultWorkspace: %v", err)
	}
	if ws2.WorkspaceID != ws1.WorkspaceID {
		t.Fatalf("expected idempotent default, got %s vs %s", ws1.WorkspaceID, ws2.WorkspaceID)
	}
}

// B6 concurrency: concurrent first-access converges on a single default record.
func TestEnsureDefaultWorkspace_ConcurrentConverges(t *testing.T) {
	s := newBindingTestStore(t)
	ctx := context.Background()
	const n = 8
	ids := make([]string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ws, err := s.EnsureDefaultWorkspace(ctx, "ten_race")
			if err != nil {
				t.Errorf("concurrent ensure: %v", err)
				return
			}
			ids[idx] = ws.WorkspaceID
		}(i)
	}
	wg.Wait()
	list, err := s.ListWorkspaces(ctx, "ten_race", 50)
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	defaults := 0
	for _, ws := range list {
		if ws.IsDefault {
			defaults++
		}
	}
	if defaults != 1 {
		t.Fatalf("expected exactly one default workspace, got %d", defaults)
	}
}

// FR-032: workspace create/archive; default workspace cannot be retired.
func TestWorkspaceLifecycle(t *testing.T) {
	s := newBindingTestStore(t)
	ctx := context.Background()
	actor := identity.TenantContext{TenantID: "ten_ws", PrincipalID: "prn_admin"}
	ws, _, err := s.CreateWorkspace(ctx, actor, "Project X")
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if ws.IsDefault {
		t.Fatalf("user workspace must not be default")
	}
	archived, _, err := s.UpdateWorkspaceStatus(ctx, actor, ws.WorkspaceID, bindings.WorkspaceArchived)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if archived.Status != bindings.WorkspaceArchived {
		t.Fatalf("expected archived, got %s", archived.Status)
	}
	def, err := s.EnsureDefaultWorkspace(ctx, "ten_ws")
	if err != nil {
		t.Fatalf("ensure default: %v", err)
	}
	if _, _, err := s.UpdateWorkspaceStatus(ctx, actor, def.WorkspaceID, bindings.WorkspaceArchived); err == nil {
		t.Fatalf("default workspace must not be retirable")
	}
}
