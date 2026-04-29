package artifacts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

type Service struct {
	dataDir        string
	billingManager *billing.Manager
	hostedBilling  bool
}

func NewService(dataDir ...string) *Service {
	root := ""
	if len(dataDir) > 0 {
		root = strings.TrimSpace(dataDir[0])
	}
	return &Service{dataDir: root}
}

func (s *Service) ConfigureBilling(manager *billing.Manager, hosted bool) {
	if s == nil {
		return
	}
	s.billingManager = manager
	s.hostedBilling = hosted
}

func (s *Service) SaveComputerUseArtifact(ctx context.Context, input computeruse.ArtifactCaptureRequest) (computeruse.Artifact, error) {
	now := time.Now().UTC()
	sum := sha256.Sum256(input.Content)
	artifactID := "cuart_" + hex.EncodeToString(sum[:8])
	storageKey := filepath.Join("computer-use", input.ComputerUseSessionID, artifactID)
	operationKey := ""
	var reservation billing.UsageReservation
	if tenantContext, ok := tenantctx.FromContext(ctx); ok && tenantContext.TenantID != "" {
		estimate := input.EstimatedByteSize
		if estimate <= 0 {
			estimate = int64(len(input.Content))
		}
		operationKey = billing.ArtifactOperationKey(tenantContext.TenantID, artifactID, storageKey, "")
		result, err := reserveArtifactStorageQuota(ctx, s.billingManager, tenantContext.TenantID, operationKey, estimate, s.hostedBilling)
		if err != nil {
			return computeruse.Artifact{}, err
		}
		reservation = result.Reservation
	}

	if strings.TrimSpace(s.dataDir) != "" {
		fullPath := filepath.Join(s.dataDir, "artifacts", storageKey)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			releaseArtifactStorageReservation(ctx, s.billingManager, reservation, "artifact directory creation failed before write")
			return computeruse.Artifact{}, fmt.Errorf("create artifact directory: %w", err)
		}
		if err := os.WriteFile(fullPath, input.Content, 0o644); err != nil {
			releaseArtifactStorageReservation(ctx, s.billingManager, reservation, "artifact content write failed before durable artifact")
			return computeruse.Artifact{}, fmt.Errorf("write artifact content: %w", err)
		}
	}

	artifact := computeruse.Artifact{
		ArtifactID:           artifactID,
		EnvironmentScope:     "",
		ComputerUseSessionID: input.ComputerUseSessionID,
		ComputerUseActionID:  input.ComputerUseActionID,
		RunID:                input.RunID,
		Kind:                 input.Kind,
		Status:               computeruse.ArtifactStatusAvailable,
		MIMEType:             strings.TrimSpace(input.MIMEType),
		FileName:             strings.TrimSpace(input.FileName),
		ByteSize:             int64(len(input.Content)),
		StorageKey:           storageKey,
		SHA256:               hex.EncodeToString(sum[:]),
		CreatedAt:            now,
		AvailableAt:          &now,
	}
	if reservation.ReservationID != "" {
		if err := commitArtifactStorageReservation(ctx, s.billingManager, reservation, operationKey, artifact.ByteSize); err != nil {
			return computeruse.Artifact{}, err
		}
	}
	return artifact, nil
}

func (s *Service) ReadComputerUseArtifactContent(_ context.Context, storageKey string) ([]byte, error) {
	if strings.TrimSpace(s.dataDir) == "" {
		return nil, nil
	}
	fullPath := filepath.Join(s.dataDir, "artifacts", filepath.Clean(storageKey))
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read artifact content: %w", err)
	}
	return content, nil
}

func reserveArtifactStorageQuota(ctx context.Context, manager *billing.Manager, tenantID, operationKey string, estimate int64, hosted bool) (billing.ReserveResult, error) {
	if manager == nil {
		if hosted {
			denial := billing.NewQuotaStateUnavailableDenial(tenantID, operationKey).Payload
			return billing.ReserveResult{Allowed: false, Denial: &denial}, billing.ErrQuotaStateUnavailable
		}
		return billing.ReserveResult{Allowed: true}, nil
	}
	return manager.Reserve(ctx, billing.ReserveInput{
		TenantID:          tenantID,
		Category:          billing.CategoryArtifactStorageBytes,
		Amount:            estimate,
		OperationKey:      operationKey,
		ReservationPoint:  "artifact write service before writing bytes using estimate",
		GuardedEntryPoint: "artifact write service",
		Hosted:            hosted,
	})
}

func releaseArtifactStorageReservation(ctx context.Context, manager *billing.Manager, reservation billing.UsageReservation, reason string) {
	if manager == nil || reservation.ReservationID == "" {
		return
	}
	_, _ = manager.Release(ctx, billing.ResolveInput{
		TenantID:     reservation.TenantID,
		Category:     reservation.Category,
		OperationKey: reservation.OperationKey,
		Amount:       reservation.AmountReserved,
		ReasonCode:   "billing.artifact_storage_released",
		Reason:       reason,
	})
}

func commitArtifactStorageReservation(ctx context.Context, manager *billing.Manager, reservation billing.UsageReservation, operationKey string, actualBytes int64) error {
	if manager == nil || reservation.ReservationID == "" {
		return nil
	}
	if operationKey == "" {
		operationKey = reservation.OperationKey
	}
	_, err := manager.Commit(ctx, billing.ResolveInput{
		TenantID:     reservation.TenantID,
		Category:     reservation.Category,
		OperationKey: operationKey,
		Amount:       actualBytes,
		ReasonCode:   "billing.artifact_storage_committed",
		Reason:       "artifact actual bytes recorded after write",
	})
	return err
}
