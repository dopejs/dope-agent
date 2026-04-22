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

	"github.com/dopejs/dope-agent/daemon/internal/computeruse"
)

type Service struct {
	dataDir string
}

func NewService(dataDir ...string) *Service {
	root := ""
	if len(dataDir) > 0 {
		root = strings.TrimSpace(dataDir[0])
	}
	return &Service{dataDir: root}
}

func (s *Service) SaveComputerUseArtifact(_ context.Context, input computeruse.ArtifactCaptureRequest) (computeruse.Artifact, error) {
	now := time.Now().UTC()
	sum := sha256.Sum256(input.Content)
	artifactID := "cuart_" + hex.EncodeToString(sum[:8])
	storageKey := filepath.Join("computer-use", input.ComputerUseSessionID, artifactID)

	if strings.TrimSpace(s.dataDir) != "" {
		fullPath := filepath.Join(s.dataDir, "artifacts", storageKey)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return computeruse.Artifact{}, fmt.Errorf("create artifact directory: %w", err)
		}
		if err := os.WriteFile(fullPath, input.Content, 0o644); err != nil {
			return computeruse.Artifact{}, fmt.Errorf("write artifact content: %w", err)
		}
	}

	return computeruse.Artifact{
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
	}, nil
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
