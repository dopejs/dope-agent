package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/managerdoc"
)

const docKindWebhook = "webhook_endpoint"

// persistedEndpoint is the durable form of a webhook: its projection plus the signing secret
// (hex) needed to verify signatures across restarts.
type persistedEndpoint struct {
	Endpoint  Endpoint `json:"endpoint"`
	SecretHex string   `json:"secretHex"`
}

var (
	ErrEndpointNotFound = errors.New("webhook endpoint not found")
	ErrInvalidEndpoint  = errors.New("webhook endpoint definition is invalid")
	ErrMissingAuth      = errors.New("webhook request is missing a signature")
	ErrBadSignature     = errors.New("webhook signature is invalid")
	ErrCrossTenant      = errors.New("webhook does not belong to the requesting tenant")
	ErrDisabled         = errors.New("webhook endpoint is disabled")
	ErrPayloadTooLarge  = errors.New("webhook payload exceeds the size limit")
	ErrQuotaDenied      = errors.New("webhook trigger denied by quota or permission")
)

// Firer fires a webhook's target (run/workflow/routine) and returns an execution reference. It
// runs only after authentication, replay, bounding, and quota checks pass.
type Firer interface {
	Fire(ctx context.Context, endpoint Endpoint, payload []byte) (executionRef string, err error)
}

// QuotaGate runs the quota/permission check before any execution starts (FR). It returns false
// plus a reason to deny.
type QuotaGate interface {
	Allow(ctx context.Context, tenantID, webhookID string) (bool, string)
}

// allowAllQuota is the default permissive gate.
type allowAllQuota struct{}

func (allowAllQuota) Allow(context.Context, string, string) (bool, string) { return true, "" }

// noopFirer records nothing and returns a synthetic reference; used when no real firer is wired.
type noopFirer struct{}

func (noopFirer) Fire(context.Context, Endpoint, []byte) (string, error) {
	return "webhook_exec_unwired", nil
}

// Manager owns webhook endpoints, verifies inbound triggers, and dispatches to the firer.
// Endpoints + secrets + replay keys are in-memory for this slice; the firer routes execution
// to the existing runtime/routine planes which own the durable execution evidence.
type Manager struct {
	mu        sync.RWMutex
	env       string
	firer     Firer
	quota     QuotaGate
	docs      managerdoc.Store
	endpoints map[string]Endpoint
	secrets   map[string][]byte          // webhookID -> signing secret (never projected)
	seenKeys  map[string]map[string]bool // webhookID -> idempotency key -> seen
}

// WithStore installs durable persistence for webhook endpoints + secrets and returns the manager.
func (m *Manager) WithStore(s managerdoc.Store) *Manager {
	m.docs = s
	return m
}

// LoadFromStore reloads persisted webhook endpoints + signing secrets on startup.
func (m *Manager) LoadFromStore(ctx context.Context) error {
	items, err := managerdoc.List[persistedEndpoint](ctx, m.docs, docKindWebhook)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range items {
		m.endpoints[item.Endpoint.WebhookID] = item.Endpoint
		if secret, decErr := hex.DecodeString(item.SecretHex); decErr == nil {
			m.secrets[item.Endpoint.WebhookID] = secret
		}
	}
	return nil
}

// persist write-throughs an endpoint + its secret. Callers hold m.mu or pass copies.
func (m *Manager) persist(endpoint Endpoint, secret []byte) {
	_ = managerdoc.Put(context.Background(), m.docs, docKindWebhook, endpoint.WebhookID, m.env, endpoint.TenantID,
		persistedEndpoint{Endpoint: endpoint, SecretHex: hex.EncodeToString(secret)})
}

func NewManager(environmentScope string, firer Firer, quota QuotaGate) *Manager {
	if firer == nil {
		firer = noopFirer{}
	}
	if quota == nil {
		quota = allowAllQuota{}
	}
	return &Manager{
		env:       strings.TrimSpace(environmentScope),
		firer:     firer,
		quota:     quota,
		endpoints: make(map[string]Endpoint),
		secrets:   make(map[string][]byte),
		seenKeys:  make(map[string]map[string]bool),
	}
}

// Create registers a webhook endpoint and returns the plaintext signing secret exactly once.
func (m *Manager) Create(tenantID, name string, targetKind TargetKind, targetRef string) (CreateSecret, error) {
	if strings.TrimSpace(tenantID) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(targetRef) == "" {
		return CreateSecret{}, ErrInvalidEndpoint
	}
	if !validTargetKind(targetKind) {
		return CreateSecret{}, ErrInvalidEndpoint
	}
	secret := randomSecret()
	now := time.Now().UTC()
	endpoint := Endpoint{
		WebhookID:         newID("webhook"),
		TenantID:          strings.TrimSpace(tenantID),
		EnvironmentScope:  m.env,
		Name:              strings.TrimSpace(name),
		TargetKind:        targetKind,
		TargetRef:         strings.TrimSpace(targetRef),
		Status:            StatusActive,
		SecretFingerprint: fingerprint(secret),
		SecretVersion:     1,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	m.mu.Lock()
	m.endpoints[endpoint.WebhookID] = endpoint
	m.secrets[endpoint.WebhookID] = secret
	m.mu.Unlock()
	m.persist(endpoint, secret)
	return CreateSecret{Endpoint: endpoint, Secret: hex.EncodeToString(secret)}, nil
}

// Rotate issues a new signing secret, invalidating the previous one.
func (m *Manager) Rotate(tenantID, webhookID string) (CreateSecret, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	endpoint, ok := m.endpoints[strings.TrimSpace(webhookID)]
	if !ok {
		return CreateSecret{}, ErrEndpointNotFound
	}
	if endpoint.TenantID != strings.TrimSpace(tenantID) {
		return CreateSecret{}, ErrCrossTenant
	}
	secret := randomSecret()
	endpoint.SecretFingerprint = fingerprint(secret)
	endpoint.SecretVersion++
	endpoint.UpdatedAt = time.Now().UTC()
	m.endpoints[endpoint.WebhookID] = endpoint
	m.secrets[endpoint.WebhookID] = secret
	m.persist(endpoint, secret)
	return CreateSecret{Endpoint: endpoint, Secret: hex.EncodeToString(secret)}, nil
}

// Disable deactivates a webhook so further triggers are rejected.
func (m *Manager) Disable(tenantID, webhookID string) (Endpoint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	endpoint, ok := m.endpoints[strings.TrimSpace(webhookID)]
	if !ok {
		return Endpoint{}, ErrEndpointNotFound
	}
	if endpoint.TenantID != strings.TrimSpace(tenantID) {
		return Endpoint{}, ErrCrossTenant
	}
	endpoint.Status = StatusDisabled
	endpoint.UpdatedAt = time.Now().UTC()
	m.endpoints[endpoint.WebhookID] = endpoint
	m.persist(endpoint, m.secrets[endpoint.WebhookID])
	return endpoint, nil
}

func (m *Manager) Get(tenantID, webhookID string) (Endpoint, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	endpoint, ok := m.endpoints[strings.TrimSpace(webhookID)]
	if !ok || endpoint.TenantID != strings.TrimSpace(tenantID) {
		return Endpoint{}, false
	}
	return endpoint, true
}

func (m *Manager) ListForTenant(tenantID string) []Endpoint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Endpoint, 0)
	for _, e := range m.endpoints {
		if e.TenantID == strings.TrimSpace(tenantID) {
			out = append(out, e)
		}
	}
	return out
}

// TriggerInput is one inbound webhook invocation.
type TriggerInput struct {
	WebhookID      string
	TenantID       string
	Signature      string // hex hmac-sha256 of payload using the signing secret
	IdempotencyKey string
	Payload        []byte
}

// Trigger authenticates and dispatches an inbound webhook. The check order is: tenant/endpoint
// resolution, status, payload bounding, signature, replay protection, quota/permission, then
// fire. Every outcome is recorded as a redacted TriggerRecord (no payload content).
func (m *Manager) Trigger(ctx context.Context, input TriggerInput) (TriggerRecord, error) {
	m.mu.Lock()
	endpoint, ok := m.endpoints[strings.TrimSpace(input.WebhookID)]
	secret := m.secrets[strings.TrimSpace(input.WebhookID)]
	m.mu.Unlock()

	record := TriggerRecord{
		TriggerID:        newID("webhook_trigger"),
		WebhookID:        strings.TrimSpace(input.WebhookID),
		TenantID:         strings.TrimSpace(input.TenantID),
		EnvironmentScope: m.env,
		IdempotencyKey:   strings.TrimSpace(input.IdempotencyKey),
		PayloadBytes:     len(input.Payload),
		CreatedAt:        time.Now().UTC(),
	}
	if !ok {
		return m.fail(record, TriggerStatusAuthFailed, ErrEndpointNotFound)
	}
	if endpoint.TenantID != strings.TrimSpace(input.TenantID) {
		return m.fail(record, TriggerStatusAuthFailed, ErrCrossTenant)
	}
	if endpoint.Status != StatusActive {
		return m.fail(record, TriggerStatusDisabled, ErrDisabled)
	}
	if len(input.Payload) > MaxPayloadBytes {
		return m.fail(record, TriggerStatusPayloadTooLarge, ErrPayloadTooLarge)
	}
	if strings.TrimSpace(input.Signature) == "" {
		return m.fail(record, TriggerStatusAuthFailed, ErrMissingAuth)
	}
	if !verifySignature(secret, input.Payload, input.Signature) {
		return m.fail(record, TriggerStatusAuthFailed, ErrBadSignature)
	}
	if key := strings.TrimSpace(input.IdempotencyKey); key != "" {
		if m.markSeen(endpoint.WebhookID, key) {
			record.Status = TriggerStatusReplaySuppressed
			return record, nil
		}
	}
	if allowed, reason := m.quota.Allow(ctx, endpoint.TenantID, endpoint.WebhookID); !allowed {
		record.FailureReason = reason
		return m.fail(record, TriggerStatusQuotaDenied, ErrQuotaDenied)
	}
	ref, err := m.firer.Fire(ctx, endpoint, input.Payload)
	if err != nil {
		return m.fail(record, TriggerStatusAuthFailed, err)
	}
	record.Status = TriggerStatusFired
	record.ExecutionRef = ref
	return record, nil
}

// TriggerSigned resolves the tenant from the (signature-authenticated) endpoint and triggers it.
// This is the inbound-ingress entry point where the request is authenticated by the signature
// rather than a bearer principal.
func (m *Manager) TriggerSigned(ctx context.Context, webhookID, signature, idempotencyKey string, payload []byte) (TriggerRecord, error) {
	m.mu.RLock()
	endpoint, ok := m.endpoints[strings.TrimSpace(webhookID)]
	m.mu.RUnlock()
	tenant := ""
	if ok {
		tenant = endpoint.TenantID
	}
	return m.Trigger(ctx, TriggerInput{WebhookID: webhookID, TenantID: tenant, Signature: signature, IdempotencyKey: idempotencyKey, Payload: payload})
}

func (m *Manager) fail(record TriggerRecord, status TriggerStatus, err error) (TriggerRecord, error) {
	record.Status = status
	if record.FailureReason == "" {
		record.FailureReason = err.Error()
	}
	return record, err
}

// markSeen records an idempotency key and reports whether it was already seen (replay).
func (m *Manager) markSeen(webhookID, key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := m.seenKeys[webhookID]
	if keys == nil {
		keys = make(map[string]bool)
		m.seenKeys[webhookID] = keys
	}
	if keys[key] {
		return true
	}
	keys[key] = true
	return false
}

func verifySignature(secret, payload []byte, signature string) bool {
	if len(secret) == 0 {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	expected := mac.Sum(nil)
	got, err := hex.DecodeString(strings.TrimSpace(signature))
	if err != nil {
		return false
	}
	return hmac.Equal(expected, got)
}

// Sign is a helper (used by clients/tests) to compute the expected signature for a payload.
func Sign(secretHex string, payload []byte) string {
	secret, err := hex.DecodeString(strings.TrimSpace(secretHex))
	if err != nil {
		return ""
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func validTargetKind(k TargetKind) bool {
	switch k {
	case TargetKindRoutine, TargetKindWorkflow, TargetKindRun:
		return true
	default:
		return false
	}
}

func randomSecret() []byte {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return []byte("insecure-fallback-secret-please-rotate")
	}
	return buf
}

func fingerprint(secret []byte) string {
	sum := sha256.Sum256(secret)
	return "sha256:" + hex.EncodeToString(sum[:])[:12]
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_fallback"
	}
	return prefix + "_" + hex.EncodeToString(buf)
}
