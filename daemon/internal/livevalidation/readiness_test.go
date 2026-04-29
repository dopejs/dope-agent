package livevalidation

import "testing"

func TestEvaluateCandidateReadinessBlocksUnsupportedClasses(t *testing.T) {
	matrix, err := NewMatrix(DefaultMatrixRows())
	if err != nil {
		t.Fatalf("NewMatrix returned error: %v", err)
	}
	result := EvaluateCandidateReadiness(matrix, CandidateReadinessInput{
		CandidateID:          "candidate_1",
		ReachableToolClasses: []ToolClass{ToolClassDaemonInspectionRead, ToolClassMCPToolCall},
		RequestedScope: SideEffectScope{
			IncludedToolClasses: []ToolClass{ToolClassDaemonInspectionRead, ToolClassMCPToolCall},
		},
	})
	if result.Status != ReadinessStatusBlocked {
		t.Fatalf("Status=%s, want blocked: %+v", result.Status, result)
	}
	if len(result.UnsupportedClasses) != 1 || result.UnsupportedClasses[0] != ToolClassMCPToolCall {
		t.Fatalf("UnsupportedClasses=%+v, want MCP only", result.UnsupportedClasses)
	}
}

func TestEvaluateCandidateReadinessAllowsMixedCandidateWhenUnsupportedExcluded(t *testing.T) {
	matrix, err := NewMatrix(DefaultMatrixRows())
	if err != nil {
		t.Fatalf("NewMatrix returned error: %v", err)
	}
	result := EvaluateCandidateReadiness(matrix, CandidateReadinessInput{
		CandidateID:          "candidate_1",
		ReachableToolClasses: []ToolClass{ToolClassDaemonInspectionRead, ToolClassMCPToolCall},
		RequestedScope: SideEffectScope{
			IncludedToolClasses: []ToolClass{ToolClassDaemonInspectionRead},
			ExcludedToolClasses: []ToolClass{ToolClassMCPToolCall},
		},
	})
	if result.Status != ReadinessStatusPartial {
		t.Fatalf("Status=%s, want partial: %+v", result.Status, result)
	}
	if len(result.RunnableToolClasses) != 1 || result.RunnableToolClasses[0] != ToolClassDaemonInspectionRead {
		t.Fatalf("RunnableToolClasses=%+v, want daemon inspection only", result.RunnableToolClasses)
	}
	var sawExcluded bool
	for _, state := range result.ToolClasses {
		if state.ToolClass == ToolClassMCPToolCall && state.Excluded {
			sawExcluded = true
		}
	}
	if !sawExcluded {
		t.Fatalf("expected unsupported MCP class to be represented as excluded: %+v", result.ToolClasses)
	}
}
