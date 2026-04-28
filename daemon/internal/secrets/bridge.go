package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	localMCPSecretsFileName   = "mcp-secrets.json"
	localSkillSecretsFileName = "skill-secrets.json"
)

type BridgeProgressStore interface {
	RegisterMigrationStep(ctx context.Context, name string) error
	IsMigrationStepCompleted(ctx context.Context, name string) (bool, error)
	BeginMigrationStep(ctx context.Context, name string) (bool, error)
	RecordMigrationChunk(ctx context.Context, name, lastProcessedKey string) error
	CompleteMigrationStep(ctx context.Context, name string) error
	FailMigrationStep(ctx context.Context, name string, cause error) error
}

type LegacyCredentialResourceStore interface {
	BridgeLegacyCredentialResources(ctx context.Context, input LegacyCredentialResourceBridgeInput) (LegacyCredentialResourceBridgeResult, error)
}

type LocalCredentialBridgeInput struct {
	DataDir       string
	TenantID      string
	StepName      string
	Manager       *Manager
	ProgressStore BridgeProgressStore
	ResourceStore LegacyCredentialResourceStore
}

type LocalCredentialBridgeResult struct {
	TenantID          string                      `json:"tenantId"`
	ScannedFiles      []string                    `json:"scannedFiles,omitempty"`
	Created           []RedactedSecretSummary     `json:"created,omitempty"`
	SkippedExisting   []RedactedSecretSummary     `json:"skippedExisting,omitempty"`
	Disabled          []DisabledResource          `json:"disabled,omitempty"`
	BridgedResources  []BridgedCredentialResource `json:"bridgedResources,omitempty"`
	DisabledResources []BridgedCredentialResource `json:"disabledResources,omitempty"`
	SecretRefCount    int                         `json:"secretRefCount"`
	CompletedAt       time.Time                   `json:"completedAt"`
	AlreadyCompleted  bool                        `json:"alreadyCompleted,omitempty"`
}

type localCredentialCandidate struct {
	secretRef string
	value     string
	sources   []string
	conflict  bool
}

func BridgeLocalCredentialFiles(ctx context.Context, input LocalCredentialBridgeInput) (LocalCredentialBridgeResult, error) {
	result := LocalCredentialBridgeResult{TenantID: strings.TrimSpace(input.TenantID)}
	if result.TenantID == "" {
		return result, ErrTenantRequired
	}
	if input.Manager == nil {
		return result, errors.New("secret manager is not configured")
	}
	if input.ProgressStore != nil && strings.TrimSpace(input.StepName) != "" {
		if err := input.ProgressStore.RegisterMigrationStep(ctx, input.StepName); err != nil {
			return result, err
		}
		completed, err := input.ProgressStore.IsMigrationStepCompleted(ctx, input.StepName)
		if err != nil {
			return result, err
		}
		if completed {
			result.AlreadyCompleted = true
			return result, nil
		}
		resume, err := input.ProgressStore.BeginMigrationStep(ctx, input.StepName)
		if err != nil {
			return result, err
		}
		if !resume {
			stepResult, err := bridgeLocalCredentialFiles(ctx, input, &result)
			if err != nil {
				_ = input.ProgressStore.FailMigrationStep(ctx, input.StepName, err)
				return stepResult, err
			}
			if err := input.ProgressStore.CompleteMigrationStep(ctx, input.StepName); err != nil {
				return stepResult, err
			}
			stepResult.CompletedAt = time.Now().UTC()
			return stepResult, nil
		}
	}
	stepResult, err := bridgeLocalCredentialFiles(ctx, input, &result)
	if err != nil {
		if input.ProgressStore != nil && strings.TrimSpace(input.StepName) != "" {
			_ = input.ProgressStore.FailMigrationStep(ctx, input.StepName, err)
		}
		return stepResult, err
	}
	if input.ProgressStore != nil && strings.TrimSpace(input.StepName) != "" {
		if err := input.ProgressStore.CompleteMigrationStep(ctx, input.StepName); err != nil {
			return stepResult, err
		}
	}
	stepResult.CompletedAt = time.Now().UTC()
	return stepResult, nil
}

func bridgeLocalCredentialFiles(ctx context.Context, input LocalCredentialBridgeInput, result *LocalCredentialBridgeResult) (LocalCredentialBridgeResult, error) {
	candidates, scanned, err := loadLocalCredentialCandidates(input.DataDir)
	if err != nil {
		return *result, err
	}
	result.ScannedFiles = scanned
	refs := make([]string, 0, len(candidates))
	for ref := range candidates {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	result.SecretRefCount = len(refs)
	for _, ref := range refs {
		candidate := candidates[ref]
		if candidate.conflict {
			secret, err := input.Manager.Get(ctx, result.TenantID, ref)
			if errors.Is(err, ErrSecretNotFound) {
				secret, err = input.Manager.CreateDisabledMetadata(ctx, CreateDisabledMetadataInput{
					TenantID:          result.TenantID,
					SecretRef:         ref,
					DisplayName:       ref,
					DisabledReason:    "legacy_secret_ref_conflict",
					RemediationReason: "move the ambiguous local credential into a tenant secret manually",
					Document: map[string]any{
						"source": "local_credential_bridge",
						"files":  append([]string(nil), candidate.sources...),
					},
				})
			}
			if err != nil {
				return *result, err
			}
			result.Disabled = append(result.Disabled, DisabledResource{
				TenantID:          result.TenantID,
				ResourceKind:      ResourceKindDisabledCredential,
				ResourceID:        secret.SecretID,
				Status:            secret.Status,
				DisabledReason:    secret.DisabledReason,
				RemediationReason: secret.RemediationReason,
				SecretRefs:        []string{ref},
				UpdatedAt:         secret.UpdatedAt,
			})
			continue
		}
		if existing, err := input.Manager.Get(ctx, result.TenantID, ref); err == nil {
			result.SkippedExisting = append(result.SkippedExisting, RedactedSecretSummary{
				SecretRef:       existing.SecretRef,
				SecretVersionID: existing.ActiveVersionID,
				Resolution:      ResolutionStatusUnavailable,
				Status:          existing.Status,
				DisabledReason:  existing.DisabledReason,
				RedactionRule:   "secret_metadata_only",
			})
			continue
		} else if !errors.Is(err, ErrSecretNotFound) {
			return *result, err
		}
		created, err := input.Manager.Create(ctx, CreateInput{
			TenantID:    result.TenantID,
			SecretRef:   ref,
			DisplayName: ref,
			Value:       candidate.value,
			Document: map[string]any{
				"source": "local_credential_bridge",
				"files":  append([]string(nil), candidate.sources...),
			},
		})
		if err != nil {
			return *result, err
		}
		result.Created = append(result.Created, RedactedSecretSummary{
			SecretRef:       created.SecretRef,
			SecretVersionID: created.ActiveVersionID,
			Resolution:      ResolutionStatusUnavailable,
			Status:          created.Status,
			RedactionRule:   "secret_metadata_only",
		})
		if input.ProgressStore != nil && strings.TrimSpace(input.StepName) != "" {
			if err := input.ProgressStore.RecordMigrationChunk(ctx, input.StepName, ref); err != nil {
				return *result, err
			}
		}
	}
	if input.ResourceStore != nil {
		legacy, err := input.ResourceStore.BridgeLegacyCredentialResources(ctx, LegacyCredentialResourceBridgeInput{
			TenantID:           result.TenantID,
			ActiveSecretRefs:   activeSecretRefs(*result),
			DisabledSecretRefs: disabledSecretRefs(*result),
		})
		if err != nil {
			return *result, err
		}
		result.BridgedResources = append(result.BridgedResources, legacy.Bridged...)
		result.DisabledResources = append(result.DisabledResources, legacy.Disabled...)
	}
	return *result, nil
}

func activeSecretRefs(result LocalCredentialBridgeResult) []string {
	seen := map[string]struct{}{}
	add := func(ref string, status SecretStatus) {
		ref = strings.TrimSpace(ref)
		if ref == "" || status != SecretStatusActive {
			return
		}
		seen[ref] = struct{}{}
	}
	for _, item := range result.Created {
		add(item.SecretRef, item.Status)
	}
	for _, item := range result.SkippedExisting {
		add(item.SecretRef, item.Status)
	}
	items := make([]string, 0, len(seen))
	for ref := range seen {
		items = append(items, ref)
	}
	sort.Strings(items)
	return items
}

func disabledSecretRefs(result LocalCredentialBridgeResult) []string {
	seen := map[string]struct{}{}
	for _, item := range result.Disabled {
		for _, ref := range item.SecretRefs {
			ref = strings.TrimSpace(ref)
			if ref != "" {
				seen[ref] = struct{}{}
			}
		}
	}
	items := make([]string, 0, len(seen))
	for ref := range seen {
		items = append(items, ref)
	}
	sort.Strings(items)
	return items
}

func loadLocalCredentialCandidates(dataDir string) (map[string]localCredentialCandidate, []string, error) {
	files := []string{localMCPSecretsFileName, localSkillSecretsFileName}
	items := make(map[string]localCredentialCandidate)
	scanned := make([]string, 0, len(files))
	for _, fileName := range files {
		values, ok, err := loadLocalCredentialFile(filepath.Join(strings.TrimSpace(dataDir), fileName))
		if err != nil {
			return nil, scanned, err
		}
		if !ok {
			continue
		}
		scanned = append(scanned, fileName)
		for key, value := range values {
			ref := strings.TrimSpace(key)
			secretValue := strings.TrimSpace(value)
			if ref == "" || secretValue == "" {
				continue
			}
			candidate := items[ref]
			if candidate.secretRef == "" {
				candidate.secretRef = ref
				candidate.value = secretValue
			} else if candidate.value != secretValue {
				candidate.conflict = true
			}
			candidate.sources = appendIfMissing(candidate.sources, fileName)
			items[ref] = candidate
		}
	}
	return items, scanned, nil
}

func loadLocalCredentialFile(path string) (map[string]string, bool, error) {
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read local credential file %s: %w", filepath.Base(path), err)
	}
	values := map[string]string{}
	if err := json.Unmarshal(payload, &values); err != nil {
		return nil, false, fmt.Errorf("decode local credential file %s: %w", filepath.Base(path), err)
	}
	return values, true, nil
}

func appendIfMissing(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}
