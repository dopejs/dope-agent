package store

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// Roadmap 35 (C4 from review) — fail-closed semantics for the
// default-personal-tenant resolver. Verifies that:
//
//  1. Cold cache + never-seeded → returns nil (downstream CHECK
//     enforces) AND warns once.
//  2. Cold cache + seeded-but-empty (legitimate pre-bootstrap) →
//     returns nil silently.
//  3. Hot cache → returns the cached id.
func TestResolveDefaultTenantBinding_FailsClosedWhenUnseeded(t *testing.T) {
	s := newBackfillTestStore(t)
	ctx := context.Background()

	// Capture the cold-path slog so we can assert exactly-one emission.
	var logBuf bytes.Buffer
	s.SetColdPathLogger(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	// 1. Cold cache, never seeded. Resolver returns nil (so the INSERT
	//    binds NULL and the schema CHECK can reject it).
	if got := s.ResolveDefaultTenantBinding(ctx); got != nil {
		t.Fatalf("cold-unseeded path: want nil, got %v", got)
	}
	// Idempotence of the warn-once: a second call still returns nil
	// AND must NOT emit a second log line.
	if got := s.ResolveDefaultTenantBinding(ctx); got != nil {
		t.Fatalf("cold-unseeded second call: want nil, got %v", got)
	}
	const wantSubstr = "ResolveDefaultTenantBinding called before SeedDefaultTenantCache"
	if got := strings.Count(logBuf.String(), wantSubstr); got != 1 {
		t.Fatalf("warn-once: expected exactly 1 log emission containing %q, got %d. Buffer:\n%s",
			wantSubstr, got, logBuf.String())
	}

	// 2. Seed the cache without a personal tenant present (matches the
	//    pre-bootstrap state). markSeeded is set internally so future
	//    calls take the silent-nil branch with NO additional log lines.
	logBuf.Reset()
	if err := s.SeedDefaultTenantCache(ctx); err != nil {
		t.Fatalf("SeedDefaultTenantCache (pre-bootstrap): %v", err)
	}
	if got := s.ResolveDefaultTenantBinding(ctx); got != nil {
		t.Fatalf("seeded-empty path: want nil, got %v", got)
	}
	if logBuf.Len() != 0 {
		t.Fatalf("seeded-empty path: expected zero log emissions, got:\n%s", logBuf.String())
	}
}

func TestResolveDefaultTenantBinding_HotCacheReturnsID(t *testing.T) {
	s := newBackfillTestStore(t)
	s.defaultTenantCache.set("ten_personal_test")
	got := s.ResolveDefaultTenantBinding(context.Background())
	if got != "ten_personal_test" {
		t.Fatalf("hot cache: want ten_personal_test, got %v", got)
	}
}

// snapshot helper exercise: after markSeeded with no id, the snapshot
// reports seeded=true id="".
func TestDefaultTenantCache_SnapshotState(t *testing.T) {
	c := &defaultTenantCache{}
	if id, seeded := c.snapshot(); id != "" || seeded {
		t.Fatalf("initial: want ('', false), got (%q, %v)", id, seeded)
	}
	c.markSeeded()
	if id, seeded := c.snapshot(); id != "" || !seeded {
		t.Fatalf("after markSeeded: want ('', true), got (%q, %v)", id, seeded)
	}
	c.set("ten_x")
	if id, seeded := c.snapshot(); id != "ten_x" || !seeded {
		t.Fatalf("after set: want ('ten_x', true), got (%q, %v)", id, seeded)
	}
}
