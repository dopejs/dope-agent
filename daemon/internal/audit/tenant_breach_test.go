package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestEmitWithoutTenantContextErrors(t *testing.T) {
	e := NewEmitter(events.NewBus(), slog.Default())
	if err := e.Emit(context.Background(), "api:GET /v1/runs/{id}", "run"); !errors.Is(err, ErrNoActingTenant) {
		t.Fatalf("expected ErrNoActingTenant, got %v", err)
	}
}

func TestEmitPayloadShape(t *testing.T) {
	bus := events.NewBus()
	ch, unsub := bus.Subscribe(events.Filter{TenantOwnedTenantID: "ten_a"})
	defer unsub()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	e := NewEmitter(bus, logger)
	ctx := tenantctx.WithContext(context.Background(), identity.TenantContext{TenantID: "ten_a", PrincipalID: "prn_x"})
	if err := e.Emit(ctx, "store:ListSchedulesForTenant", "schedule"); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	select {
	case ev := <-ch:
		if ev.Category != EventCategory || ev.Name != EventName {
			t.Fatalf("envelope mismatch: %s/%s", ev.Category, ev.Name)
		}
		if ev.Resource.Kind != "tenant" || ev.Resource.ID != "ten_a" {
			t.Fatalf("resource mismatch: %+v", ev.Resource)
		}
		if ev.TenantID != "ten_a" {
			t.Fatalf("event tenant_id %q", ev.TenantID)
		}
		// Payload must contain exactly the four declared fields and no
		// target tenant id or row data.
		got := ev.Payload
		want := map[string]any{
			"actingTenantId": "ten_a",
			"principalId":    "prn_x",
			"surface":        "store:ListSchedulesForTenant",
			"resourceKind":   "schedule",
		}
		if len(got) != len(want) {
			t.Fatalf("payload size mismatch: %d != %d (%v)", len(got), len(want), got)
		}
		for k, v := range want {
			if got[k] != v {
				t.Fatalf("payload[%s] = %v, want %v", k, got[k], v)
			}
		}
		for k := range got {
			if _, ok := want[k]; !ok {
				t.Fatalf("payload contains forbidden key %q", k)
			}
		}
		// Defensive: no field anywhere in the payload should be a target
		// tenant id (e.g. "ten_b" would never appear because the API
		// never accepts one as input).
	default:
		t.Fatalf("subscriber did not receive audit event")
	}

	// Structured log assertion: the JSON-encoded line must include the
	// stable code and the four fields.
	lines := strings.Split(strings.TrimSpace(logBuf.String()), "\n")
	if len(lines) == 0 {
		t.Fatalf("no log lines emitted")
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &rec); err != nil {
		t.Fatalf("log line not JSON: %v: %s", err, lines[len(lines)-1])
	}
	if rec["msg"] != LogCode {
		t.Fatalf("log msg = %v, want %s", rec["msg"], LogCode)
	}
	for _, key := range []string{"acting_tenant_id", "principal_id", "surface", "resource_kind"} {
		if _, ok := rec[key]; !ok {
			t.Fatalf("log line missing field %q", key)
		}
	}
}
