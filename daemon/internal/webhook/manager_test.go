package webhook

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

type recordingFirer struct{ fired int }

func (f *recordingFirer) Fire(context.Context, Endpoint, []byte) (string, error) {
	f.fired++
	return "exec_1", nil
}

type denyQuota struct{}

func (denyQuota) Allow(context.Context, string, string) (bool, string) {
	return false, "monthly webhook quota exhausted"
}

func setup(t *testing.T) (*Manager, *recordingFirer, Endpoint, string) {
	t.Helper()
	firer := &recordingFirer{}
	m := NewManager("test", firer, nil)
	created, err := m.Create("ten_a", "deploy hook", TargetKindRoutine, "routine_1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return m, firer, created.Endpoint, created.Secret
}

// US1 + FR: a correctly-signed trigger fires the target exactly once.
func TestWebhookTriggerFires(t *testing.T) {
	m, firer, ep, secret := setup(t)
	payload := []byte(`{"event":"push"}`)
	rec, err := m.Trigger(context.Background(), TriggerInput{
		WebhookID: ep.WebhookID, TenantID: "ten_a", Signature: Sign(secret, payload),
		IdempotencyKey: "evt-1", Payload: payload,
	})
	if err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if rec.Status != TriggerStatusFired || rec.ExecutionRef == "" || firer.fired != 1 {
		t.Fatalf("trigger did not fire cleanly: rec=%+v fired=%d", rec, firer.fired)
	}
	// FR redaction: the record carries size, not payload content.
	if rec.PayloadBytes != len(payload) {
		t.Fatalf("payload bytes wrong: %d", rec.PayloadBytes)
	}
}

// Security matrix (FR): missing auth, bad signature, replay, oversized, cross-tenant, disabled,
// quota denial all reject without firing.
func TestWebhookSecurityMatrix(t *testing.T) {
	m, firer, ep, secret := setup(t)
	payload := []byte(`{"event":"push"}`)
	good := Sign(secret, payload)

	// missing auth
	if rec, err := m.Trigger(context.Background(), TriggerInput{WebhookID: ep.WebhookID, TenantID: "ten_a", Payload: payload}); !errors.Is(err, ErrMissingAuth) || rec.Status != TriggerStatusAuthFailed {
		t.Fatalf("missing auth not rejected: rec=%+v err=%v", rec, err)
	}
	// bad signature
	if _, err := m.Trigger(context.Background(), TriggerInput{WebhookID: ep.WebhookID, TenantID: "ten_a", Signature: "deadbeef", Payload: payload}); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("bad signature not rejected: %v", err)
	}
	// cross-tenant
	if _, err := m.Trigger(context.Background(), TriggerInput{WebhookID: ep.WebhookID, TenantID: "ten_b", Signature: good, Payload: payload}); !errors.Is(err, ErrCrossTenant) {
		t.Fatalf("cross-tenant not rejected: %v", err)
	}
	// oversized payload
	big := bytes.Repeat([]byte("x"), MaxPayloadBytes+1)
	if _, err := m.Trigger(context.Background(), TriggerInput{WebhookID: ep.WebhookID, TenantID: "ten_a", Signature: Sign(secret, big), Payload: big}); !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("oversized payload not rejected: %v", err)
	}
	if firer.fired != 0 {
		t.Fatalf("no failed trigger should fire the target, fired=%d", firer.fired)
	}

	// replay protection: same idempotency key fires once, second is suppressed.
	if rec, _ := m.Trigger(context.Background(), TriggerInput{WebhookID: ep.WebhookID, TenantID: "ten_a", Signature: good, IdempotencyKey: "k1", Payload: payload}); rec.Status != TriggerStatusFired {
		t.Fatalf("first keyed trigger should fire: %+v", rec)
	}
	if rec, err := m.Trigger(context.Background(), TriggerInput{WebhookID: ep.WebhookID, TenantID: "ten_a", Signature: good, IdempotencyKey: "k1", Payload: payload}); err != nil || rec.Status != TriggerStatusReplaySuppressed {
		t.Fatalf("replay not suppressed: rec=%+v err=%v", rec, err)
	}
	if firer.fired != 1 {
		t.Fatalf("replay must not re-fire: fired=%d", firer.fired)
	}

	// disabled endpoint rejects.
	if _, err := m.Disable("ten_a", ep.WebhookID); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if _, err := m.Trigger(context.Background(), TriggerInput{WebhookID: ep.WebhookID, TenantID: "ten_a", Signature: good, IdempotencyKey: "k2", Payload: payload}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled endpoint not rejected: %v", err)
	}
}

// FR: quota/permission check runs before execution.
func TestWebhookQuotaDenialBeforeFire(t *testing.T) {
	firer := &recordingFirer{}
	m := NewManager("test", firer, denyQuota{})
	created, _ := m.Create("ten_a", "hook", TargetKindWorkflow, "summarize")
	payload := []byte(`{}`)
	rec, err := m.Trigger(context.Background(), TriggerInput{WebhookID: created.Endpoint.WebhookID, TenantID: "ten_a", Signature: Sign(created.Secret, payload), Payload: payload})
	if !errors.Is(err, ErrQuotaDenied) || rec.Status != TriggerStatusQuotaDenied {
		t.Fatalf("quota not enforced before fire: rec=%+v err=%v", rec, err)
	}
	if firer.fired != 0 {
		t.Fatalf("quota-denied trigger must not fire: %d", firer.fired)
	}
}

// FR: rotating the secret invalidates the previous signature; the secret is never projected.
func TestWebhookRotateInvalidatesOldSecret(t *testing.T) {
	m, _, ep, oldSecret := setup(t)
	payload := []byte(`{"a":1}`)
	rotated, err := m.Rotate("ten_a", ep.WebhookID)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if rotated.Endpoint.SecretVersion != 2 || rotated.Endpoint.SecretFingerprint == ep.SecretFingerprint {
		t.Fatalf("rotate did not change secret: %+v", rotated.Endpoint)
	}
	if _, err := m.Trigger(context.Background(), TriggerInput{WebhookID: ep.WebhookID, TenantID: "ten_a", Signature: Sign(oldSecret, payload), Payload: payload}); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("old secret should be invalid after rotate: %v", err)
	}
	if _, err := m.Trigger(context.Background(), TriggerInput{WebhookID: ep.WebhookID, TenantID: "ten_a", Signature: Sign(rotated.Secret, payload), Payload: payload}); err != nil {
		t.Fatalf("new secret should be valid: %v", err)
	}
}
