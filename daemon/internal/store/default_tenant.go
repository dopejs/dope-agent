package store

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
)

// testRunDiscard is a logger sink for use under `go test`. The C4
// cold-path warning is operationally critical in production but during
// unit tests many helper stores (sandbox, scheduler, computeruse
// fixtures, etc.) construct *SQLiteStore directly and never call
// SeedDefaultTenantCache. Those tests are not exercising the real boot
// path, so suppressing the slog noise keeps test output clean while
// preserving the warn-once semantics for production. Any test that
// wants to assert the warning fires can override via SetColdPathLogger.
var testRunDiscard = slog.New(slog.NewTextHandler(io.Discard, nil))

// Roadmap 35 (Pass B) — default-personal-tenant resolver.
//
// The single-operator daemon always has exactly one `personal` tenant
// after BootstrapLocal runs at app boot. Pass A wired tenant-aware
// helpers that bind tenant_id atomically when the request context
// carries a resolved TenantContext, but ~25 legacy callers persist
// runtime spine rows without one (background loops, scheduler dispatch,
// IM message ingestion, evaluation replay recorder, checkpoint
// persistence, etc.). Patching every caller to plumb through a tenant
// context is a substantial cross-package refactor; the pragmatic Pass B
// move is to have the legacy Upsert helpers themselves resolve the
// default personal tenant when no context-carried tenant id is present.
//
// This file owns that resolver. It caches the bootstrapped personal
// tenant id on first read so the hot path is a single map lookup.
//
// Invariant: resolveDefaultPersonalTenantID returns "" only when no
// personal tenant exists yet (pre-bootstrap NewSQLiteStore boot path).
// Every Upsert* call AFTER bootstrap finds a non-empty id.

// ErrDefaultPersonalTenantUnavailable signals that the legacy Upsert
// helper was invoked before the personal tenant was bootstrapped AND
// the caller did not supply a tenant context. Callers that observe
// this should ensure BootstrapLocal has run before invoking the
// runtime spine writers.
var ErrDefaultPersonalTenantUnavailable = errors.New("store: default personal tenant not bootstrapped yet")

// defaultTenantCache holds the resolved personal tenant id. We cache
// because it never changes after BootstrapLocal seeds it (the
// single-operator system has exactly one personal tenant for life).
//
// `seeded` records whether SeedDefaultTenantCache has been invoked
// successfully at least once. This separates "no personal tenant
// exists yet" (legitimate pre-bootstrap state, seeded=true id="") from
// "we never tried to look up the personal tenant" (forgotten boot
// step, fail-closed). Used by ResolveDefaultTenantBinding to refuse
// silent cold-path binding when the schema is post-enforcement.
type defaultTenantCache struct {
	mu     sync.RWMutex
	id     string
	seeded bool
	// coldWarnedOnce ensures the operator-visible warning fires at
	// most once per process for a missed-seeding bug. Repeated logs
	// would just spam without surfacing new information.
	coldWarnedOnce atomic.Bool
}

func (c *defaultTenantCache) get() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.id
}

// getOK returns (id, true) only when the cache has been seeded with a
// non-empty id. Distinguishes "no cache yet" from "cached empty" so
// callers like AppendEvent can decide whether to bind tenant_id or
// leave it NULL on the cold path.
func (c *defaultTenantCache) getOK() (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.id == "" {
		return "", false
	}
	return c.id, true
}

// snapshot returns the current id and whether the cache has been
// seeded. Used by ResolveDefaultTenantBinding to enforce fail-closed
// semantics on the cold path.
func (c *defaultTenantCache) snapshot() (id string, seeded bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.id, c.seeded
}

func (c *defaultTenantCache) set(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.id = id
	c.seeded = true
}

// markSeeded records that SeedDefaultTenantCache ran to completion,
// even if no personal tenant was found. Without this, the resolver
// would not be able to distinguish "no tenant yet" from "forgot to
// seed" and would have to fail-closed on every cold-cache call.
func (c *defaultTenantCache) markSeeded() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seeded = true
}

// ResolveDefaultTenantBinding returns the resolved default-personal
// tenant id as a value suitable for direct sql.DB binding, or `nil` when
// no personal tenant has been bootstrapped yet. Used by the extended
// Pass B legacy Upsert helpers (T019–T026) to bind `tenant_id` inline in
// their existing INSERT...ON CONFLICT statements:
//
//	INSERT INTO X (..., tenant_id) VALUES (..., ?)
//	ON CONFLICT(pk) DO UPDATE SET
//	  ...,
//	  tenant_id = COALESCE(X.tenant_id, excluded.tenant_id)
//
// The COALESCE guard preserves any pre-bound tenant_id so a legacy
// caller cannot accidentally hijack a row that was previously written
// through a tenant-aware path.
//
// Pre-bootstrap (the very first NewSQLiteStore boot path before
// BootstrapLocal seeds the personal tenant) the helper returns nil; the
// row is then inserted with tenant_id=NULL which is permitted by the
// schema until step (c) enforcement runs. By the time enforcement runs
// BootstrapLocal has already completed and SeedDefaultTenantCache has
// populated the cache, so the resolver returns a non-empty id without
// ever issuing SQL. This matters because the resolver may run inside an
// active transaction (e.g. ReplaceWorkflowSteps); the SQLite store is
// configured with MaxOpenConns=1, so any SQL issued while a tx holds
// the connection would deadlock.
func (s *SQLiteStore) ResolveDefaultTenantBinding(ctx context.Context) any {
	if s == nil {
		return nil
	}
	id, seeded := s.defaultTenantCache.snapshot()
	if id != "" {
		return id
	}
	if seeded {
		// Seeding ran but found no personal tenant. Pre-bootstrap state
		// is the only legitimate cause; the caller's INSERT will write
		// tenant_id=NULL which the schema allowed pre-T077a/b. Post-
		// enforcement the column-level NOT NULL CHECK fails the INSERT
		// loudly, which is the desired fail-closed signal.
		return nil
	}
	// Fail-closed cold path: SeedDefaultTenantCache was never invoked
	// for this store, so we cannot tell whether a personal tenant
	// exists. Issuing SQL here would deadlock under MaxOpenConns(1) +
	// transaction-bound callers (see ReplaceWorkflowSteps), so we log
	// a critical warning once and return nil. The downstream T077b
	// CHECK constraint or the per-table NOT NULL enforcement will
	// reject the resulting INSERT, surfacing the missed-seeding bug
	// at the boundary it was supposed to guard.
	if s.defaultTenantCache.coldWarnedOnce.CompareAndSwap(false, true) {
		logger := s.coldPathLogger
		if logger == nil {
			if testing.Testing() {
				logger = testRunDiscard
			} else {
				logger = slog.Default()
			}
		}
		logger.Error(
			"store: ResolveDefaultTenantBinding called before SeedDefaultTenantCache; "+
				"writes will return NULL tenant_id and may be rejected by the T077a/b "+
				"NOT NULL/CHECK enforcement. App boot must call SeedDefaultTenantCache "+
				"after BootstrapLocal.",
			"path", "store.ResolveDefaultTenantBinding",
		)
	}
	return nil
}

// SeedDefaultTenantCache reads the personal tenant id and primes the
// in-memory cache so that ResolveDefaultTenantBinding can answer purely
// from memory afterwards. Called from the app boot path immediately
// after BootstrapLocal completes. Safe to call repeatedly; a no-op once
// the cache is populated.
func (s *SQLiteStore) SeedDefaultTenantCache(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if s.defaultTenantCache.get() != "" {
		return nil
	}
	_, err := s.ResolveDefaultPersonalTenantID(ctx)
	if errors.Is(err, ErrDefaultPersonalTenantUnavailable) {
		// "No personal tenant yet" is a legitimate pre-bootstrap state
		// — record that we did look so ResolveDefaultTenantBinding's
		// fail-closed branch isn't tripped on the next call.
		s.defaultTenantCache.markSeeded()
		return nil
	}
	return err
}

// ResolveDefaultPersonalTenantID returns the bootstrapped personal
// tenant id, caching after the first read. Returns "" only when no
// personal tenant has been bootstrapped (pre-app.New boot path).
//
// Used by the legacy Upsert helpers to bind tenant_id when the caller
// did not supply a tenant context — this lets the runtime-spine
// NOT NULL/CHECK enforcement (T077a) activate without auditing every
// caller individually.
func (s *SQLiteStore) ResolveDefaultPersonalTenantID(ctx context.Context) (string, error) {
	if s == nil {
		return "", nil
	}
	if cached := s.defaultTenantCache.get(); cached != "" {
		return cached, nil
	}
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT tenant_id FROM tenants WHERE tenant_kind = 'personal' ORDER BY created_at ASC LIMIT 1`).Scan(&id)
	if err != nil {
		// sql.ErrNoRows is the legitimate pre-bootstrap state; surface
		// the typed error so callers can decide whether to skip the
		// auto-bind or surface a clear failure.
		return "", ErrDefaultPersonalTenantUnavailable
	}
	if id == "" {
		return "", ErrDefaultPersonalTenantUnavailable
	}
	s.defaultTenantCache.set(id)
	return id, nil
}
