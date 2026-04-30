package evaluation

import (
	"fmt"
	"strings"
	"time"
)

const (
	DiscoveryPartialReasonMaxInspectedRecords  = "max_inspected_records"
	DiscoveryPartialReasonMaxEmittedCandidates = "max_emitted_candidates"
)

type StartDiscoveryRunInput struct {
	WindowStart          time.Time
	WindowEnd            time.Time
	SourceKinds          []SourceKind
	MaxInspectedRecords  int
	MaxEmittedCandidates int
	CostBudget           int
	Cursor               string
	StartedBy            string
	IdempotencyKey       string
}

type DiscoveryProgress struct {
	InspectedRecords  int
	EmittedCandidates int
	Cursor            string
	Completed         bool
	FailedReason      string
}

func BuildDiscoveryRunFromPolicy(policy DiscoveryPolicy, input StartDiscoveryRunInput, now time.Time) (DiscoveryRun, error) {
	policy = mergeDiscoveryPolicyInput(policy, input)
	if err := ValidateDiscoveryPolicy(policy); err != nil {
		return DiscoveryRun{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	runID := "discovery_run_" + strings.ReplaceAll(DiscoveryIdempotencyScope(DiscoveryRun{TenantID: policy.TenantID, IdempotencyKey: input.IdempotencyKey}), ":", "_")
	if strings.TrimSpace(input.IdempotencyKey) == "" {
		runID = fmt.Sprintf("discovery_run_%d", now.UnixNano())
	}
	return DiscoveryRun{
		DiscoveryRunID:       runID,
		TenantID:             policy.TenantID,
		PolicyID:             policy.PolicyID,
		Status:               ProductStatusQueued,
		Cursor:               input.Cursor,
		SourceKinds:          append([]SourceKind(nil), policy.SourceKinds...),
		WindowStart:          policy.WindowStart,
		WindowEnd:            policy.WindowEnd,
		MaxInspectedRecords:  policy.MaxInspectedRecords,
		MaxEmittedCandidates: policy.MaxEmittedCandidates,
		CostBudget:           policy.CostBudget,
		StartedBy:            input.StartedBy,
		StartedAt:            now.UTC(),
		UpdatedAt:            now.UTC(),
		IdempotencyKey:       input.IdempotencyKey,
	}, nil
}

func ApplyDiscoveryRunProgress(run DiscoveryRun, progress DiscoveryProgress, now time.Time) DiscoveryRun {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	run.InspectedRecords += progress.InspectedRecords
	run.EmittedCandidates += progress.EmittedCandidates
	if strings.TrimSpace(progress.Cursor) != "" {
		run.Cursor = progress.Cursor
	}
	run.UpdatedAt = now.UTC()
	if strings.TrimSpace(progress.FailedReason) != "" {
		run.Status = ProductStatusFailed
		run.PartialReason = progress.FailedReason
		completedAt := now.UTC()
		run.CompletedAt = &completedAt
		return run
	}
	if run.MaxInspectedRecords > 0 && run.InspectedRecords >= run.MaxInspectedRecords {
		run.Status = ProductStatusPartial
		run.PartialReason = DiscoveryPartialReasonMaxInspectedRecords
		completedAt := now.UTC()
		run.CompletedAt = &completedAt
		return run
	}
	if run.MaxEmittedCandidates > 0 && run.EmittedCandidates >= run.MaxEmittedCandidates {
		run.Status = ProductStatusPartial
		run.PartialReason = DiscoveryPartialReasonMaxEmittedCandidates
		completedAt := now.UTC()
		run.CompletedAt = &completedAt
		return run
	}
	if progress.Completed {
		run.Status = ProductStatusCompleted
		completedAt := now.UTC()
		run.CompletedAt = &completedAt
	}
	return run
}

func DiscoveryIdempotencyScope(run DiscoveryRun) string {
	if strings.TrimSpace(run.TenantID) == "" || strings.TrimSpace(run.IdempotencyKey) == "" {
		return ""
	}
	return strings.TrimSpace(run.TenantID) + ":" + strings.TrimSpace(run.IdempotencyKey)
}

func mergeDiscoveryPolicyInput(policy DiscoveryPolicy, input StartDiscoveryRunInput) DiscoveryPolicy {
	if !input.WindowStart.IsZero() {
		policy.WindowStart = input.WindowStart
	}
	if !input.WindowEnd.IsZero() {
		policy.WindowEnd = input.WindowEnd
	}
	if len(input.SourceKinds) > 0 {
		policy.SourceKinds = append([]SourceKind(nil), input.SourceKinds...)
	}
	if input.MaxInspectedRecords > 0 {
		policy.MaxInspectedRecords = input.MaxInspectedRecords
	}
	if input.MaxEmittedCandidates > 0 {
		policy.MaxEmittedCandidates = input.MaxEmittedCandidates
	}
	if input.CostBudget > 0 {
		policy.CostBudget = input.CostBudget
	}
	return policy
}
