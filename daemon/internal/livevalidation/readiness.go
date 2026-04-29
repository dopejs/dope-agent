package livevalidation

type ReadinessStatus string

const (
	ReadinessStatusReady   ReadinessStatus = "ready"
	ReadinessStatusPartial ReadinessStatus = "partial"
	ReadinessStatusBlocked ReadinessStatus = "blocked"
)

type CandidateReadinessInput struct {
	CandidateID          string
	ReachableToolClasses []ToolClass
	RequestedScope       SideEffectScope
}

type ToolClassReadiness struct {
	ToolClass  ToolClass  `json:"toolClass"`
	Status     string     `json:"status"`
	Excluded   bool       `json:"excluded"`
	MatrixRow  *MatrixRow `json:"matrixRow,omitempty"`
	ReasonCode string     `json:"reasonCode,omitempty"`
	Message    string     `json:"message,omitempty"`
}

type CandidateReadinessResult struct {
	CandidateID         string               `json:"candidateId"`
	Status              ReadinessStatus      `json:"status"`
	ToolClasses         []ToolClassReadiness `json:"toolClasses"`
	UnsupportedClasses  []ToolClass          `json:"unsupportedClasses,omitempty"`
	RunnableToolClasses []ToolClass          `json:"runnableToolClasses,omitempty"`
}

func EvaluateCandidateReadiness(matrix Matrix, input CandidateReadinessInput) CandidateReadinessResult {
	result := CandidateReadinessResult{CandidateID: input.CandidateID, Status: ReadinessStatusReady}
	included := toolClassSet(input.RequestedScope.IncludedToolClasses)
	excluded := toolClassSet(input.RequestedScope.ExcludedToolClasses)
	hasExplicitIncludes := len(included) > 0

	for _, toolClass := range input.ReachableToolClasses {
		if excluded[toolClass] {
			row, ok := matrix.Row(toolClass)
			if !ok || row.SafetyClass == SafetyClassUnsupported {
				result.UnsupportedClasses = append(result.UnsupportedClasses, toolClass)
				state := ToolClassReadiness{
					ToolClass:  toolClass,
					Status:     "excluded",
					Excluded:   true,
					ReasonCode: "live_validation.unsupported_excluded",
					Message:    "Unsupported tool class is explicitly excluded from live validation.",
				}
				if ok {
					state.MatrixRow = &row
				}
				result.ToolClasses = append(result.ToolClasses, state)
				result.Status = maxReadiness(result.Status, ReadinessStatusPartial)
				continue
			}
			result.ToolClasses = append(result.ToolClasses, ToolClassReadiness{
				ToolClass:  toolClass,
				Status:     "excluded",
				Excluded:   true,
				MatrixRow:  &row,
				ReasonCode: "live_validation.scope_excluded",
				Message:    "Tool class is outside the requested live-validation scope.",
			})
			result.Status = maxReadiness(result.Status, ReadinessStatusPartial)
			continue
		}
		if hasExplicitIncludes && !included[toolClass] {
			row, ok := matrix.Row(toolClass)
			if !ok {
				result.UnsupportedClasses = append(result.UnsupportedClasses, toolClass)
				result.ToolClasses = append(result.ToolClasses, ToolClassReadiness{
					ToolClass:  toolClass,
					Status:     "unsupported",
					ReasonCode: "live_validation.matrix_row_missing",
					Message:    "Unsupported tool class outside the included scope must be explicitly excluded.",
				})
				result.Status = ReadinessStatusBlocked
				continue
			}
			if row.SafetyClass == SafetyClassUnsupported {
				result.UnsupportedClasses = append(result.UnsupportedClasses, toolClass)
				result.ToolClasses = append(result.ToolClasses, ToolClassReadiness{
					ToolClass:  toolClass,
					Status:     "unsupported",
					MatrixRow:  &row,
					ReasonCode: "live_validation.unsupported_tool_class",
					Message:    "Unsupported tool class outside the included scope must be explicitly excluded.",
				})
				result.Status = ReadinessStatusBlocked
				continue
			}
			result.ToolClasses = append(result.ToolClasses, ToolClassReadiness{
				ToolClass:  toolClass,
				Status:     "excluded",
				Excluded:   true,
				ReasonCode: "live_validation.scope_excluded",
				Message:    "Tool class is outside the requested live-validation scope.",
			})
			continue
		}
		row, ok := matrix.Row(toolClass)
		if !ok {
			result.UnsupportedClasses = append(result.UnsupportedClasses, toolClass)
			result.ToolClasses = append(result.ToolClasses, ToolClassReadiness{
				ToolClass:  toolClass,
				Status:     "unsupported",
				ReasonCode: "live_validation.matrix_row_missing",
				Message:    "Tool class has no replay support matrix row.",
			})
			result.Status = ReadinessStatusBlocked
			continue
		}
		if row.SafetyClass == SafetyClassUnsupported {
			result.UnsupportedClasses = append(result.UnsupportedClasses, toolClass)
			result.ToolClasses = append(result.ToolClasses, ToolClassReadiness{
				ToolClass:  toolClass,
				Status:     "unsupported",
				MatrixRow:  &row,
				ReasonCode: "live_validation.unsupported_tool_class",
				Message:    "Tool class is unsupported for live validation.",
			})
			result.Status = ReadinessStatusBlocked
			continue
		}
		rowCopy := row
		result.RunnableToolClasses = append(result.RunnableToolClasses, toolClass)
		result.ToolClasses = append(result.ToolClasses, ToolClassReadiness{
			ToolClass: toolClass,
			Status:    "supported",
			MatrixRow: &rowCopy,
		})
	}
	return result
}

func toolClassSet(items []ToolClass) map[ToolClass]bool {
	set := make(map[ToolClass]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

func maxReadiness(current, next ReadinessStatus) ReadinessStatus {
	if current == ReadinessStatusBlocked || next == ReadinessStatusBlocked {
		return ReadinessStatusBlocked
	}
	if current == ReadinessStatusPartial || next == ReadinessStatusPartial {
		return ReadinessStatusPartial
	}
	return ReadinessStatusReady
}
