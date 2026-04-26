package audit_test

// Roadmap 35 (US3 / T085) — cross-tenant audit integration.
//
// Exercises the audit pipeline end-to-end: each test case stages a
// tenant_owned row under tenant A, then performs a cross-tenant access
// from tenant B through the production tenancy helper for that resource.
// Every denied access MUST:
//
//  1. Produce exactly ONE bus event with envelope
//     category=audit, name=audit.cross_tenant_access_denied,
//     resource.kind=tenant, resource.id=<actingTenantId>.
//  2. Carry a payload with EXACTLY four declared keys
//     (actingTenantId, principalId, surface, resourceKind) and NO target
//     tenant id, target resource id, or row data.
//  3. Validate against the JSON schema constraints
//     (schemas/events/audit-cross-tenant-access-denied.event.schema.json):
//     id pattern, payload key set, principalId pattern, surface format.
//  4. Emit one structured log line whose `msg` is the stable code
//     `audit.cross_tenant_access_denied` and whose attributes mirror the
//     payload.
//
// The cases cover every `resourceKind` the runtime-spine tenancy helpers
// can deny today (T064/T077a). Extended-domain helpers will join this
// table as their cross-tenant audit emission is wired (T077a per-table
// rollout); the table-driven shape lets new entries be added one line at
// a time.

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/calendar"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/evaluation"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
	"github.com/dopejs/dope-agent/daemon/internal/policy"
	"github.com/dopejs/dope-agent/daemon/internal/router"
	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
	"github.com/dopejs/dope-agent/daemon/internal/store/tenancy"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

const (
	tenantAID    = "ten_aaaa1111"
	tenantBID    = "ten_bbbb2222"
	principalAID = "prn_aaaa1111"
	principalBID = "prn_bbbb2222"
)

var (
	// Patterns mirror the JSON schema for the audit event payload.
	tenantIDPattern    = regexp.MustCompile(`^ten_[A-Za-z0-9]+$`)
	principalIDPattern = regexp.MustCompile(`^prn_[A-Za-z0-9]+$`)
	allowedPayloadKeys = map[string]struct{}{
		"actingTenantId": {},
		"principalId":    {},
		"surface":        {},
		"resourceKind":   {},
	}
)

func TestAuditCrossTenantIntegration(t *testing.T) {
	cases := []auditCase{
		{
			name:         "runs_get",
			resourceKind: "run",
			surface:      "store:GetRunForTenant",
			seed: func(t *testing.T, rt *tenancy.Runtime, ctxA context.Context) {
				now := time.Now().UTC()
				if err := rt.UpsertRunForTenant(ctxA, runtime.Run{
					RunID: "run_T085_a", Entrypoint: "test", Status: runtime.RunStatusQueued,
					Goal: "isolation", CreatedAt: now, UpdatedAt: now,
				}); err != nil {
					t.Fatalf("seed run: %v", err)
				}
			},
			act: func(t *testing.T, rt *tenancy.Runtime, ctxB context.Context) {
				_, _, _ = rt.GetRunForTenant(ctxB, "run_T085_a")
			},
		},
		{
			name:         "runs_upsert_collision",
			resourceKind: "run",
			surface:      "store:UpsertRunForTenant",
			seed: func(t *testing.T, rt *tenancy.Runtime, ctxA context.Context) {
				now := time.Now().UTC()
				if err := rt.UpsertRunForTenant(ctxA, runtime.Run{
					RunID: "run_T085_b", Entrypoint: "test", Status: runtime.RunStatusQueued,
					Goal: "collision", CreatedAt: now, UpdatedAt: now,
				}); err != nil {
					t.Fatalf("seed run: %v", err)
				}
			},
			act: func(t *testing.T, rt *tenancy.Runtime, ctxB context.Context) {
				now := time.Now().UTC()
				_ = rt.UpsertRunForTenant(ctxB, runtime.Run{
					RunID: "run_T085_b", Entrypoint: "x", Status: runtime.RunStatusQueued,
					Goal: "hijack", CreatedAt: now, UpdatedAt: now,
				})
			},
		},
		{
			name:         "runs_delete",
			resourceKind: "run",
			surface:      "store:DeleteRunForTenant",
			seed: func(t *testing.T, rt *tenancy.Runtime, ctxA context.Context) {
				now := time.Now().UTC()
				if err := rt.UpsertRunForTenant(ctxA, runtime.Run{
					RunID: "run_T085_c", Entrypoint: "test", Status: runtime.RunStatusQueued,
					Goal: "delete", CreatedAt: now, UpdatedAt: now,
				}); err != nil {
					t.Fatalf("seed run: %v", err)
				}
			},
			act: func(t *testing.T, rt *tenancy.Runtime, ctxB context.Context) {
				_, _ = rt.DeleteRunForTenant(ctxB, "run_T085_c")
			},
		},
		{
			name:         "sessions_upsert_collision",
			resourceKind: "session",
			surface:      "store:UpsertSessionForTenant",
			seed: func(t *testing.T, rt *tenancy.Runtime, ctxA context.Context) {
				now := time.Now().UTC()
				if err := rt.UpsertSessionForTenant(ctxA, router.Session{
					SessionID: "sess_T085_a", Kind: "chat", Status: "active",
					Channel: "test", PeerID: "peer", RoutingKey: "rk_T085_a", Generation: 1,
					CreatedAt: now, UpdatedAt: now, LastActiveAt: now,
				}); err != nil {
					t.Fatalf("seed session: %v", err)
				}
			},
			act: func(t *testing.T, rt *tenancy.Runtime, ctxB context.Context) {
				now := time.Now().UTC()
				_ = rt.UpsertSessionForTenant(ctxB, router.Session{
					SessionID: "sess_T085_a", Kind: "chat", Status: "active",
					Channel: "test", PeerID: "peer", RoutingKey: "rk_T085_x", Generation: 2,
					CreatedAt: now, UpdatedAt: now, LastActiveAt: now,
				})
			},
		},
		{
			name:         "llm_dispatches_get",
			resourceKind: "llm_dispatch",
			surface:      "store:GetLLMDispatchForTenant",
			seed: func(t *testing.T, rt *tenancy.Runtime, ctxA context.Context) {
				now := time.Now().UTC()
				if err := rt.UpsertLLMDispatchForTenant(ctxA, llm.Dispatch{
					DispatchID: "disp_T085_a", Provider: "echo", Model: "echo-v1",
					Messages: []llm.Message{{Role: "user", Content: "hi"}},
					Status:   llm.DispatchStatusCompleted,
					Usage:    llm.Usage{TotalTokens: 1},
					CreatedAt: now, UpdatedAt: now,
				}); err != nil {
					t.Fatalf("seed dispatch: %v", err)
				}
			},
			act: func(t *testing.T, rt *tenancy.Runtime, ctxB context.Context) {
				_, _, _ = rt.GetLLMDispatchForTenant(ctxB, "disp_T085_a")
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			runAuditCase(t, c)
		})
	}
}

type auditCase struct {
	name         string
	resourceKind string
	surface      string
	seed         func(t *testing.T, rt *tenancy.Runtime, ctxA context.Context)
	act          func(t *testing.T, rt *tenancy.Runtime, ctxB context.Context)
}

func runAuditCase(t *testing.T, c auditCase) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	bus := events.NewBus()
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	emitter := audit.NewEmitter(bus, logger)
	rt := tenancy.NewRuntime(s, emitter)

	ch, unsub := bus.Subscribe(events.Filter{Category: audit.EventCategory})
	t.Cleanup(unsub)

	ctxA := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: tenantAID, PrincipalID: principalAID})
	ctxB := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: tenantBID, PrincipalID: principalBID})

	c.seed(t, rt, ctxA)
	// Drain any audit events produced by seeding (none expected, but
	// make the assertion tight).
	drainAuditChannel(ch)
	logBuf.Reset()

	c.act(t, rt, ctxB)

	var got events.Event
	select {
	case got = <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s: no audit event received", c.name)
	}
	// No second event — exactly-one semantics.
	select {
	case extra := <-ch:
		t.Fatalf("%s: received unexpected second audit event: %+v", c.name, extra)
	case <-time.After(50 * time.Millisecond):
	}

	assertAuditEnvelope(t, c, got)
	assertAuditPayload(t, c, got)
	assertAuditLogLine(t, c, logBuf.String())
}

func drainAuditChannel(ch <-chan events.Event) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func assertAuditEnvelope(t *testing.T, c auditCase, ev events.Event) {
	t.Helper()
	if ev.Category != audit.EventCategory {
		t.Errorf("%s: envelope.category=%q, want %q", c.name, ev.Category, audit.EventCategory)
	}
	if ev.Name != audit.EventName {
		t.Errorf("%s: envelope.name=%q, want %q", c.name, ev.Name, audit.EventName)
	}
	if ev.Resource.Kind != "tenant" {
		t.Errorf("%s: envelope.resource.kind=%q, want tenant", c.name, ev.Resource.Kind)
	}
	if ev.Resource.ID != tenantBID {
		t.Errorf("%s: envelope.resource.id=%q, want acting tenant %q", c.name, ev.Resource.ID, tenantBID)
	}
	if !tenantIDPattern.MatchString(ev.Resource.ID) {
		t.Errorf("%s: envelope.resource.id=%q does not match %s", c.name, ev.Resource.ID, tenantIDPattern)
	}
	// Defensive: target tenant id MUST NOT appear in the envelope.
	if ev.Resource.ID == tenantAID {
		t.Errorf("%s: envelope.resource.id leaks target tenant id", c.name)
	}
	if ev.TenantID == tenantAID {
		t.Errorf("%s: event.tenant_id leaks target tenant id", c.name)
	}
}

func assertAuditPayload(t *testing.T, c auditCase, ev events.Event) {
	t.Helper()
	if len(ev.Payload) != len(allowedPayloadKeys) {
		t.Errorf("%s: payload has %d keys, want %d (%v)", c.name, len(ev.Payload), len(allowedPayloadKeys), keysOf(ev.Payload))
	}
	for k := range ev.Payload {
		if _, ok := allowedPayloadKeys[k]; !ok {
			t.Errorf("%s: payload contains forbidden key %q", c.name, k)
		}
	}
	if got, _ := ev.Payload["actingTenantId"].(string); got != tenantBID {
		t.Errorf("%s: payload.actingTenantId=%q, want %q", c.name, got, tenantBID)
	}
	if got, _ := ev.Payload["principalId"].(string); got != principalBID {
		t.Errorf("%s: payload.principalId=%q, want %q", c.name, got, principalBID)
	}
	if got, _ := ev.Payload["principalId"].(string); !principalIDPattern.MatchString(got) {
		t.Errorf("%s: payload.principalId=%q does not match %s", c.name, got, principalIDPattern)
	}
	if got, _ := ev.Payload["surface"].(string); got != c.surface {
		t.Errorf("%s: payload.surface=%q, want %q", c.name, got, c.surface)
	}
	if got, _ := ev.Payload["resourceKind"].(string); got != c.resourceKind {
		t.Errorf("%s: payload.resourceKind=%q, want %q", c.name, got, c.resourceKind)
	}
	// Schema requires surface format "<kind>:<route_or_helper>".
	if surface, _ := ev.Payload["surface"].(string); !strings.Contains(surface, ":") {
		t.Errorf("%s: payload.surface=%q lacks `<kind>:<route>` format", c.name, surface)
	}
	// Defensive: payload values MUST NOT mention target tenant id or
	// target resource id directly. The four declared fields are by design
	// either acting-tenant or stable identifiers; but if a future change
	// added a new payload field that included a target id, this test
	// catches it via the strict key-set check above.
	for k, v := range ev.Payload {
		s, ok := v.(string)
		if !ok {
			continue
		}
		if s == tenantAID {
			t.Errorf("%s: payload[%s] leaks target tenant id %q", c.name, k, s)
		}
	}
}

func assertAuditLogLine(t *testing.T, c auditCase, raw string) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec["msg"] != audit.LogCode {
			continue
		}
		for _, key := range []string{"acting_tenant_id", "principal_id", "surface", "resource_kind"} {
			if _, ok := rec[key]; !ok {
				t.Errorf("%s: log line missing field %q", c.name, key)
			}
		}
		if got, _ := rec["acting_tenant_id"].(string); got != tenantBID {
			t.Errorf("%s: log acting_tenant_id=%q, want %q", c.name, got, tenantBID)
		}
		if got, _ := rec["resource_kind"].(string); got != c.resourceKind {
			t.Errorf("%s: log resource_kind=%q, want %q", c.name, got, c.resourceKind)
		}
		return
	}
	t.Errorf("%s: no structured log line with msg=%q found in:\n%s", c.name, audit.LogCode, raw)
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Roadmap 35 (post-review I6) — extended audit integration coverage.
//
// The original TestAuditCrossTenantIntegration above only exercises the
// runtime-spine helpers (runs, sessions, llm_dispatches). I6 expands to
// every other domain that owns a tenant-aware helper: schedules,
// calendar, mail, reminders, computer-use, evaluation. The structural
// invariants (envelope, payload key set, log line shape) are identical
// to the runtime-spine cases, so the asserter helpers are reused.
type extendedHarnesses struct {
	store       *store.SQLiteStore
	runtime     *tenancy.Runtime
	schedules   *tenancy.Schedules
	calendar    *tenancy.Calendar
	mail        *tenancy.Mail
	reminders   *tenancy.Reminders
	computerUse *tenancy.ComputerUse
	evaluation  *tenancy.Evaluation
	approvals   *tenancy.Approvals
}

type extendedAuditCase struct {
	name         string
	resourceKind string
	surface      string
	seed         func(t *testing.T, h extendedHarnesses, ctxA context.Context)
	act          func(t *testing.T, h extendedHarnesses, ctxB context.Context)
}

func TestAuditCrossTenantIntegrationExtendedDomains(t *testing.T) {
	now := time.Now().UTC()
	cases := []extendedAuditCase{
		{
			name:         "schedules_upsert_collision",
			resourceKind: "schedule",
			surface:      "store:UpsertScheduleForTenant",
			seed: func(t *testing.T, h extendedHarnesses, ctxA context.Context) {
				if err := h.schedules.UpsertScheduleForTenant(ctxA, store.ScheduleRecord{
					ScheduleID: "sch_T085_a", EnvironmentScope: "test", Kind: "one_time",
					Status: "scheduled", TargetRefID: "tgt_a",
					CreatedAt: now, UpdatedAt: now, Document: []byte(`{}`),
				}); err != nil {
					t.Fatalf("seed schedule: %v", err)
				}
			},
			act: func(t *testing.T, h extendedHarnesses, ctxB context.Context) {
				_ = h.schedules.UpsertScheduleForTenant(ctxB, store.ScheduleRecord{
					ScheduleID: "sch_T085_a", EnvironmentScope: "test", Kind: "one_time",
					Status: "scheduled", TargetRefID: "tgt_hijack",
					CreatedAt: now, UpdatedAt: now, Document: []byte(`{}`),
				})
			},
		},
		{
			name:         "calendar_account_upsert_collision",
			resourceKind: "calendar_account",
			surface:      "store:UpsertCalendarAccountForTenant",
			seed: func(t *testing.T, h extendedHarnesses, ctxA context.Context) {
				if err := h.calendar.UpsertAccountForTenant(ctxA, calendar.AccountProjection{
					CalendarAccountID: "cal_T085_a", IntegrationID: "int_a",
					DomainKind: "google", EnvironmentScope: "test",
					ReadinessStatus: "ready", PrimaryCalendarRef: "primary",
					PrimaryCalendarLabel: "Primary", PrimaryTimezone: "UTC",
					LastSyncedAt: now, UpdatedAt: now,
				}); err != nil {
					t.Fatalf("seed calendar account: %v", err)
				}
			},
			act: func(t *testing.T, h extendedHarnesses, ctxB context.Context) {
				_ = h.calendar.UpsertAccountForTenant(ctxB, calendar.AccountProjection{
					CalendarAccountID: "cal_T085_a", IntegrationID: "int_a",
					DomainKind: "google", EnvironmentScope: "test",
					ReadinessStatus: "ready", PrimaryCalendarRef: "primary",
					PrimaryCalendarLabel: "Primary", PrimaryTimezone: "UTC",
					LastSyncedAt: now, UpdatedAt: now,
				})
			},
		},
		{
			name:         "mail_account_upsert_collision",
			resourceKind: "mail_account",
			surface:      "store:UpsertMailAccountForTenant",
			seed: func(t *testing.T, h extendedHarnesses, ctxA context.Context) {
				if err := h.mail.UpsertAccountForTenant(ctxA, mail.AccountProjection{
					MailAccountID: "mail_T085_a", IntegrationID: "int_a",
					DomainKind: "google", EnvironmentScope: "test",
					ReadinessStatus: "ready", MailboxAddress: "a@example.com",
					MailboxLabel: "A", LastSyncedAt: now, UpdatedAt: now,
				}); err != nil {
					t.Fatalf("seed mail account: %v", err)
				}
			},
			act: func(t *testing.T, h extendedHarnesses, ctxB context.Context) {
				_ = h.mail.UpsertAccountForTenant(ctxB, mail.AccountProjection{
					MailAccountID: "mail_T085_a", IntegrationID: "int_a",
					DomainKind: "google", EnvironmentScope: "test",
					ReadinessStatus: "ready", MailboxAddress: "a@example.com",
					MailboxLabel: "A", LastSyncedAt: now, UpdatedAt: now,
				})
			},
		},
		{
			name:         "reminders_upsert_collision",
			resourceKind: "reminder",
			surface:      "store:UpsertReminderForTenant",
			seed: func(t *testing.T, h extendedHarnesses, ctxA context.Context) {
				if err := h.reminders.UpsertReminderForTenant(ctxA, store.ReminderRecord{
					ReminderID: "rem_T085_a", EnvironmentScope: "test",
					BehaviorMode: "one_time", CurrentState: "scheduled",
					UpdatedAt: now, Document: []byte(`{}`),
				}); err != nil {
					t.Fatalf("seed reminder: %v", err)
				}
			},
			act: func(t *testing.T, h extendedHarnesses, ctxB context.Context) {
				_ = h.reminders.UpsertReminderForTenant(ctxB, store.ReminderRecord{
					ReminderID: "rem_T085_a", EnvironmentScope: "test",
					BehaviorMode: "one_time", CurrentState: "scheduled",
					UpdatedAt: now, Document: []byte(`{}`),
				})
			},
		},
		{
			name:         "computer_use_session_upsert_collision",
			resourceKind: "computer_use_session",
			surface:      "store:UpsertComputerUseSessionForTenant",
			seed: func(t *testing.T, h extendedHarnesses, ctxA context.Context) {
				// computer_use_sessions has a runtime FK on runs(run_id);
				// seed the parent run via the tenant-aware Runtime helper
				// so the FK resolves under tenant A.
				if err := h.runtime.UpsertRunForTenant(ctxA, runtime.Run{
					RunID: "run_for_cu_T085", Entrypoint: "test", Status: runtime.RunStatusQueued,
					Goal: "cu", CreatedAt: now, UpdatedAt: now,
				}); err != nil {
					t.Fatalf("seed parent run: %v", err)
				}
				if err := h.computerUse.UpsertSessionForTenant(ctxA, computeruse.Session{
					ComputerUseSessionID: "cus_T085_a",
					EnvironmentScope:     "test",
					RunID:                "run_for_cu_T085",
					Status:               computeruse.SessionStatusActive,
					DriverKind:           "browser",
					StartedAt:            now,
					UpdatedAt:            now,
				}); err != nil {
					t.Fatalf("seed computer-use session: %v", err)
				}
			},
			act: func(t *testing.T, h extendedHarnesses, ctxB context.Context) {
				_ = h.computerUse.UpsertSessionForTenant(ctxB, computeruse.Session{
					ComputerUseSessionID: "cus_T085_a",
					EnvironmentScope:     "test",
					RunID:                "run_for_cu_T085",
					Status:               computeruse.SessionStatusActive,
					DriverKind:           "browser",
					StartedAt:            now,
					UpdatedAt:            now,
				})
			},
		},
		{
			name:         "approvals_upsert_collision",
			resourceKind: "approval",
			surface:      "store:UpsertApprovalForTenant",
			seed: func(t *testing.T, h extendedHarnesses, ctxA context.Context) {
				if err := h.approvals.UpsertApprovalForTenant(ctxA, policy.Approval{
					ApprovalID: "apv_T085_a",
					Action:     "sandbox.exec",
					Reason:     "isolation",
					Status:     policy.ApprovalStatusPending,
					CreatedAt:  now, UpdatedAt: now,
				}); err != nil {
					t.Fatalf("seed approval: %v", err)
				}
			},
			act: func(t *testing.T, h extendedHarnesses, ctxB context.Context) {
				_ = h.approvals.UpsertApprovalForTenant(ctxB, policy.Approval{
					ApprovalID: "apv_T085_a",
					Action:     "sandbox.exec",
					Reason:     "hijack",
					Status:     policy.ApprovalStatusPending,
					CreatedAt:  now, UpdatedAt: now,
				})
			},
		},
		{
			name:         "decisions_upsert_collision",
			resourceKind: "decision",
			surface:      "store:UpsertDecisionForTenant",
			seed: func(t *testing.T, h extendedHarnesses, ctxA context.Context) {
				if err := h.approvals.UpsertDecisionForTenant(ctxA, policy.Decision{
					DecisionID: "dec_T085_a",
					Action:     "sandbox.exec",
					Outcome:    policy.DecisionOutcomeApproved,
					Reason:     "isolation",
					CreatedAt:  now,
				}); err != nil {
					t.Fatalf("seed decision: %v", err)
				}
			},
			act: func(t *testing.T, h extendedHarnesses, ctxB context.Context) {
				_ = h.approvals.UpsertDecisionForTenant(ctxB, policy.Decision{
					DecisionID: "dec_T085_a",
					Action:     "sandbox.exec",
					Outcome:    policy.DecisionOutcomeRejected,
					Reason:     "hijack",
					CreatedAt:  now,
				})
			},
		},
		{
			name:         "evaluation_regression_fixture_upsert_collision",
			resourceKind: "evaluation_regression_fixture",
			surface:      "store:UpsertRegressionFixtureForTenant",
			seed: func(t *testing.T, h extendedHarnesses, ctxA context.Context) {
				if err := h.evaluation.UpsertRegressionFixtureForTenant(ctxA, evaluation.RegressionFixture{
					FixtureID:        "fix_T085_a",
					ManifestPath:     "fixtures/T085_a.json",
					EnvironmentScope: "test",
					CreatedAt:        now,
					UpdatedAt:        now,
				}); err != nil {
					t.Fatalf("seed regression fixture: %v", err)
				}
			},
			act: func(t *testing.T, h extendedHarnesses, ctxB context.Context) {
				_ = h.evaluation.UpsertRegressionFixtureForTenant(ctxB, evaluation.RegressionFixture{
					FixtureID:        "fix_T085_a",
					ManifestPath:     "fixtures/T085_a.json",
					EnvironmentScope: "test",
					CreatedAt:        now,
					UpdatedAt:        now,
				})
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			runExtendedAuditCase(t, c)
		})
	}
}

func runExtendedAuditCase(t *testing.T, c extendedAuditCase) {
	s, err := store.NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	// Match the daemon boot order so legacy Upsert helpers don't trip
	// the fail-closed cold-path warning from C4.
	if err := s.SeedDefaultTenantCache(context.Background()); err != nil {
		t.Fatalf("SeedDefaultTenantCache: %v", err)
	}

	bus := events.NewBus()
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	emitter := audit.NewEmitter(bus, logger)

	h := extendedHarnesses{
		store:       s,
		runtime:     tenancy.NewRuntime(s, emitter),
		schedules:   tenancy.NewSchedules(s, emitter),
		calendar:    tenancy.NewCalendar(s, emitter),
		mail:        tenancy.NewMail(s, emitter),
		reminders:   tenancy.NewReminders(s, emitter),
		computerUse: tenancy.NewComputerUse(s, emitter),
		evaluation:  tenancy.NewEvaluation(s, emitter),
		approvals:   tenancy.NewApprovals(s, emitter),
	}

	ch, unsub := bus.Subscribe(events.Filter{Category: audit.EventCategory})
	t.Cleanup(unsub)

	ctxA := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: tenantAID, PrincipalID: principalAID})
	ctxB := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: tenantBID, PrincipalID: principalBID})

	c.seed(t, h, ctxA)
	drainAuditChannel(ch)
	logBuf.Reset()

	c.act(t, h, ctxB)

	var got events.Event
	select {
	case got = <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s: no audit event received", c.name)
	}
	select {
	case extra := <-ch:
		t.Fatalf("%s: received unexpected second audit event: %+v", c.name, extra)
	case <-time.After(50 * time.Millisecond):
	}

	// Reuse the runtime-spine asserters by adapting to the existing
	// auditCase struct (envelope/payload/log invariants are identical
	// across all domains by contract).
	adapter := auditCase{
		name:         c.name,
		resourceKind: c.resourceKind,
		surface:      c.surface,
	}
	assertAuditEnvelope(t, adapter, got)
	assertAuditPayload(t, adapter, got)
	assertAuditLogLine(t, adapter, logBuf.String())
}
