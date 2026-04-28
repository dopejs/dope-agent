package secrets

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Store interface {
	CreateSecret(ctx context.Context, secret TenantSecret, version SecretVersion) error
	UpdateSecretMetadata(ctx context.Context, secret TenantSecret) error
	RotateSecret(ctx context.Context, secret TenantSecret, previousVersionID string, version SecretVersion) error
	DisableSecret(ctx context.Context, secret TenantSecret) error
	GetSecretByRef(ctx context.Context, tenantID, secretRef string) (TenantSecret, bool, error)
	GetSecretVersion(ctx context.Context, tenantID, secretVersionID string) (SecretVersion, bool, error)
	ListSecrets(ctx context.Context, tenantID string) ([]TenantSecret, error)
}

type Manager struct {
	store   Store
	backend ValueBackend
	now     func() time.Time
	mu      sync.Mutex
}

func NewManager(store Store, backend ValueBackend) *Manager {
	return &Manager{
		store:   store,
		backend: backend,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func (m *Manager) Create(ctx context.Context, input CreateInput) (TenantSecret, error) {
	if err := validateSecretInput(input.TenantID, input.SecretRef, input.Value); err != nil {
		return TenantSecret{}, err
	}
	if m == nil || m.store == nil || m.backend == nil {
		return TenantSecret{}, errors.New("secret manager is not configured")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok, err := m.store.GetSecretByRef(ctx, input.TenantID, input.SecretRef); err != nil {
		return TenantSecret{}, err
	} else if ok {
		return existing, fmt.Errorf("tenant secret already exists: %s", input.SecretRef)
	}

	now := m.now()
	secretID := "sec_" + randomHex(12)
	versionID := "secver_" + randomHex(12)
	backendRef, err := m.backend.Put(ctx, input.TenantID, secretID, versionID, input.Value)
	if err != nil {
		return TenantSecret{}, err
	}
	activatedAt := now
	secret := TenantSecret{
		SecretID:        secretID,
		TenantID:        input.TenantID,
		SecretRef:       strings.TrimSpace(input.SecretRef),
		DisplayName:     input.DisplayName,
		Status:          SecretStatusActive,
		ActiveVersionID: versionID,
		CreatedAt:       now,
		UpdatedAt:       now,
		RotatedAt:       &now,
		Document:        cloneDocument(input.Document),
	}
	version := SecretVersion{
		SecretVersionID: versionID,
		SecretID:        secretID,
		TenantID:        input.TenantID,
		SecretRef:       strings.TrimSpace(input.SecretRef),
		VersionNumber:   1,
		Status:          SecretVersionStatusActive,
		ValueBackendRef: backendRef,
		CreatedAt:       now,
		ActivatedAt:     &activatedAt,
	}
	if err := m.store.CreateSecret(ctx, secret, version); err != nil {
		_ = m.backend.Delete(ctx, backendRef)
		return TenantSecret{}, err
	}
	return secret, nil
}

func (m *Manager) List(ctx context.Context, tenantID string) ([]TenantSecret, error) {
	if tenantID == "" {
		return nil, ErrTenantRequired
	}
	if m == nil || m.store == nil {
		return nil, errors.New("secret manager is not configured")
	}
	return m.store.ListSecrets(ctx, tenantID)
}

func (m *Manager) Get(ctx context.Context, tenantID, secretRef string) (TenantSecret, error) {
	if tenantID == "" {
		return TenantSecret{}, ErrTenantRequired
	}
	if strings.TrimSpace(secretRef) == "" {
		return TenantSecret{}, ErrSecretRefRequired
	}
	if m == nil || m.store == nil {
		return TenantSecret{}, errors.New("secret manager is not configured")
	}
	secret, ok, err := m.store.GetSecretByRef(ctx, tenantID, secretRef)
	if err != nil {
		return TenantSecret{}, err
	}
	if !ok {
		return TenantSecret{}, ErrSecretNotFound
	}
	return secret, nil
}

func (m *Manager) UpdateMetadata(ctx context.Context, input UpdateMetadataInput) (TenantSecret, error) {
	if input.TenantID == "" {
		return TenantSecret{}, ErrTenantRequired
	}
	if strings.TrimSpace(input.SecretRef) == "" {
		return TenantSecret{}, ErrSecretRefRequired
	}
	if m == nil || m.store == nil {
		return TenantSecret{}, errors.New("secret manager is not configured")
	}
	secret, ok, err := m.store.GetSecretByRef(ctx, input.TenantID, input.SecretRef)
	if err != nil {
		return TenantSecret{}, err
	}
	if !ok {
		return TenantSecret{}, ErrSecretNotFound
	}
	if input.DisplayName != nil {
		secret.DisplayName = *input.DisplayName
	}
	if input.Document != nil {
		secret.Document = cloneDocument(input.Document)
	}
	secret.UpdatedAt = m.now()
	if err := m.store.UpdateSecretMetadata(ctx, secret); err != nil {
		return TenantSecret{}, err
	}
	return secret, nil
}

func (m *Manager) Rotate(ctx context.Context, input RotateInput) (TenantSecret, error) {
	if err := validateSecretInput(input.TenantID, input.SecretRef, input.Value); err != nil {
		return TenantSecret{}, err
	}
	if m == nil || m.store == nil || m.backend == nil {
		return TenantSecret{}, errors.New("secret manager is not configured")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	secret, ok, err := m.store.GetSecretByRef(ctx, input.TenantID, input.SecretRef)
	if err != nil {
		return TenantSecret{}, err
	}
	if !ok {
		return TenantSecret{}, ErrSecretNotFound
	}
	if secret.Status != SecretStatusActive {
		return TenantSecret{}, ErrSecretDisabled
	}
	previousVersionID := secret.ActiveVersionID
	now := m.now()
	versionID := "secver_" + randomHex(12)
	backendRef, err := m.backend.Put(ctx, input.TenantID, secret.SecretID, versionID, input.Value)
	if err != nil {
		return TenantSecret{}, err
	}
	version := SecretVersion{
		SecretVersionID: versionID,
		SecretID:        secret.SecretID,
		TenantID:        secret.TenantID,
		SecretRef:       secret.SecretRef,
		VersionNumber:   0,
		Status:          SecretVersionStatusActive,
		ValueBackendRef: backendRef,
		CreatedAt:       now,
		ActivatedAt:     &now,
	}
	secret.ActiveVersionID = versionID
	secret.RotatedAt = &now
	secret.UpdatedAt = now
	if err := m.store.RotateSecret(ctx, secret, previousVersionID, version); err != nil {
		_ = m.backend.Delete(ctx, backendRef)
		return TenantSecret{}, err
	}
	return secret, nil
}

func (m *Manager) Disable(ctx context.Context, input DisableInput) (TenantSecret, error) {
	if input.TenantID == "" {
		return TenantSecret{}, ErrTenantRequired
	}
	if strings.TrimSpace(input.SecretRef) == "" {
		return TenantSecret{}, ErrSecretRefRequired
	}
	if m == nil || m.store == nil {
		return TenantSecret{}, errors.New("secret manager is not configured")
	}
	secret, ok, err := m.store.GetSecretByRef(ctx, input.TenantID, input.SecretRef)
	if err != nil {
		return TenantSecret{}, err
	}
	if !ok {
		return TenantSecret{}, ErrSecretNotFound
	}
	now := m.now()
	secret.Status = SecretStatusDisabled
	secret.DisabledReason = input.DisabledReason
	secret.DisabledAt = &now
	secret.UpdatedAt = now
	if err := m.store.DisableSecret(ctx, secret); err != nil {
		return TenantSecret{}, err
	}
	return secret, nil
}

func (m *Manager) CreateDisabledMetadata(ctx context.Context, input CreateDisabledMetadataInput) (TenantSecret, error) {
	if input.TenantID == "" {
		return TenantSecret{}, ErrTenantRequired
	}
	if strings.TrimSpace(input.SecretRef) == "" {
		return TenantSecret{}, ErrSecretRefRequired
	}
	if m == nil || m.store == nil {
		return TenantSecret{}, errors.New("secret manager is not configured")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok, err := m.store.GetSecretByRef(ctx, input.TenantID, input.SecretRef); err != nil {
		return TenantSecret{}, err
	} else if ok {
		return existing, fmt.Errorf("tenant secret already exists: %s", input.SecretRef)
	}

	now := m.now()
	secretID := "sec_" + randomHex(12)
	versionID := "secver_" + randomHex(12)
	secret := TenantSecret{
		SecretID:          secretID,
		TenantID:          input.TenantID,
		SecretRef:         strings.TrimSpace(input.SecretRef),
		DisplayName:       input.DisplayName,
		Status:            SecretStatusPendingRemediation,
		ActiveVersionID:   versionID,
		DisabledReason:    input.DisabledReason,
		RemediationReason: input.RemediationReason,
		CreatedAt:         now,
		UpdatedAt:         now,
		DisabledAt:        &now,
		Document:          cloneDocument(input.Document),
	}
	version := SecretVersion{
		SecretVersionID: versionID,
		SecretID:        secretID,
		TenantID:        input.TenantID,
		SecretRef:       strings.TrimSpace(input.SecretRef),
		VersionNumber:   1,
		Status:          SecretVersionStatusPendingRemediation,
		CreatedAt:       now,
	}
	if err := m.store.CreateSecret(ctx, secret, version); err != nil {
		return TenantSecret{}, err
	}
	return secret, nil
}

func (m *Manager) Resolve(ctx context.Context, input ResolveInput) (ResolvedSecret, error) {
	if input.TenantID == "" {
		return ResolvedSecret{}, ErrTenantRequired
	}
	if strings.TrimSpace(input.SecretRef) == "" {
		return ResolvedSecret{}, ErrSecretRefRequired
	}
	if m == nil || m.store == nil || m.backend == nil {
		return ResolvedSecret{}, errors.New("secret manager is not configured")
	}
	secret, ok, err := m.store.GetSecretByRef(ctx, input.TenantID, input.SecretRef)
	if err != nil {
		return ResolvedSecret{}, err
	}
	if !ok {
		return ResolvedSecret{}, ErrSecretNotFound
	}
	if secret.TenantID != input.TenantID {
		return ResolvedSecret{}, ErrCrossTenantSecret
	}
	if secret.Status != SecretStatusActive {
		return ResolvedSecret{}, ErrSecretDisabled
	}
	version, ok, err := m.store.GetSecretVersion(ctx, input.TenantID, secret.ActiveVersionID)
	if err != nil {
		return ResolvedSecret{}, err
	}
	if !ok {
		return ResolvedSecret{}, ErrSecretVersionNotFound
	}
	value, err := m.backend.Get(ctx, version.ValueBackendRef)
	if err != nil {
		return ResolvedSecret{}, err
	}
	return ResolvedSecret{
		TenantID:        input.TenantID,
		SecretID:        secret.SecretID,
		SecretRef:       secret.SecretRef,
		SecretVersionID: version.SecretVersionID,
		Resolution:      ResolutionStatusResolved,
		Value:           value,
		ResolvedAt:      m.now(),
	}, nil
}

func validateSecretInput(tenantID, secretRef, value string) error {
	if tenantID == "" {
		return ErrTenantRequired
	}
	if strings.TrimSpace(secretRef) == "" {
		return ErrSecretRefRequired
	}
	if value == "" {
		return ErrSecretValueRequired
	}
	return nil
}

func cloneDocument(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}
