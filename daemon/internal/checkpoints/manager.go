package checkpoints

import (
	"context"
	"fmt"

	"github.com/dopejs/dope-agent/daemon/internal/runtime"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type RecoveryStats struct {
	RunCount int
}

type Manager struct {
	store   *store.SQLiteStore
	runtime *runtime.Manager
}

func NewManager(sqliteStore *store.SQLiteStore, runtimeManager *runtime.Manager) *Manager {
	return &Manager{
		store:   sqliteStore,
		runtime: runtimeManager,
	}
}

func (m *Manager) SaveRunCheckpoint(ctx context.Context, runID string) error {
	if m == nil || m.store == nil || m.runtime == nil {
		return nil
	}

	checkpoint, err := m.runtime.SnapshotRun(runID)
	if err != nil {
		return fmt.Errorf("snapshot run %s: %w", runID, err)
	}
	if err := m.persistSnapshotState(ctx, checkpoint); err != nil {
		return fmt.Errorf("persist checkpoint state for run %s: %w", runID, err)
	}

	if err := m.store.SaveCheckpoint(ctx, checkpoint); err != nil {
		return fmt.Errorf("save checkpoint for run %s: %w", runID, err)
	}

	return nil
}

func (m *Manager) persistSnapshotState(ctx context.Context, checkpoint runtime.RunCheckpoint) error {
	if err := m.store.UpsertRun(ctx, checkpoint.Run); err != nil {
		return fmt.Errorf("upsert checkpoint run %s: %w", checkpoint.Run.RunID, err)
	}
	for _, step := range checkpoint.Steps {
		if err := m.store.UpsertStep(ctx, step); err != nil {
			return fmt.Errorf("upsert checkpoint step %s: %w", step.StepID, err)
		}
	}
	for _, toolCall := range checkpoint.ToolCalls {
		if err := m.store.UpsertToolCall(ctx, toolCall); err != nil {
			return fmt.Errorf("upsert checkpoint tool call %s: %w", toolCall.ToolCallID, err)
		}
	}
	return nil
}

func (m *Manager) RestoreRunCheckpoint(ctx context.Context, checkpoint runtime.RunCheckpoint) error {
	if m == nil || m.store == nil || m.runtime == nil {
		return nil
	}

	m.runtime.RestoreRunCheckpoint(checkpoint)
	if err := m.persistSnapshotState(ctx, checkpoint); err != nil {
		return fmt.Errorf("persist restored checkpoint state for run %s: %w", checkpoint.Run.RunID, err)
	}
	if err := m.store.SaveCheckpoint(ctx, checkpoint); err != nil {
		return fmt.Errorf("save restored checkpoint for run %s: %w", checkpoint.Run.RunID, err)
	}

	return nil
}

func (m *Manager) Restore(ctx context.Context) (RecoveryStats, error) {
	if m == nil || m.store == nil || m.runtime == nil {
		return RecoveryStats{}, nil
	}

	checkpoints, err := m.store.ListLatestCheckpoints(ctx)
	if err != nil {
		return RecoveryStats{}, fmt.Errorf("load latest checkpoints: %w", err)
	}

	m.runtime.RestoreCheckpoints(checkpoints)

	return RecoveryStats{
		RunCount: len(checkpoints),
	}, nil
}

func (m *Manager) Close() error {
	return nil
}
