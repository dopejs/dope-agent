//! Candidate readiness evaluation (port of `readiness.go`).

use std::collections::HashSet;

use serde::Deserialize;
use serde::Serialize;

use crate::matrix::Matrix;
use crate::matrix::MatrixRow;
use crate::matrix::SafetyClass;
use crate::matrix::ToolClass;
use crate::types::SideEffectScope;

define_string_enum!(
    /// Overall readiness verdict for a candidate.
    ReadinessStatus {
        READY => "ready",
        PARTIAL => "partial",
        BLOCKED => "blocked"
    }
);

/// Input to [`evaluate_candidate_readiness`].
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CandidateReadinessInput {
    pub candidate_id: String,
    pub reachable_tool_classes: Vec<ToolClass>,
    pub requested_scope: SideEffectScope,
}

/// Per-tool-class readiness state.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ToolClassReadiness {
    pub tool_class: ToolClass,
    pub status: String,
    pub excluded: bool,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub matrix_row: Option<MatrixRow>,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub reason_code: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub message: String,
}

/// The readiness verdict for one candidate.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CandidateReadinessResult {
    pub candidate_id: String,
    pub status: ReadinessStatus,
    pub tool_classes: Vec<ToolClassReadiness>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub unsupported_classes: Vec<ToolClass>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub runnable_tool_classes: Vec<ToolClass>,
}

/// Builds the tool-class membership set used for include/exclude checks.
pub(crate) fn tool_class_set(items: &[ToolClass]) -> HashSet<ToolClass> {
    items.iter().cloned().collect()
}

/// Port of `maxReadiness`.
fn max_readiness(current: &ReadinessStatus, next: &ReadinessStatus) -> ReadinessStatus {
    if current.as_str() == ReadinessStatus::BLOCKED || next.as_str() == ReadinessStatus::BLOCKED {
        return ReadinessStatus::from(ReadinessStatus::BLOCKED);
    }
    if current.as_str() == ReadinessStatus::PARTIAL || next.as_str() == ReadinessStatus::PARTIAL {
        return ReadinessStatus::from(ReadinessStatus::PARTIAL);
    }
    ReadinessStatus::from(ReadinessStatus::READY)
}

/// Port of `EvaluateCandidateReadiness`.
#[must_use]
pub fn evaluate_candidate_readiness(
    matrix: &Matrix,
    input: CandidateReadinessInput,
) -> CandidateReadinessResult {
    let mut result = CandidateReadinessResult {
        candidate_id: input.candidate_id,
        status: ReadinessStatus::from(ReadinessStatus::READY),
        ..CandidateReadinessResult::default()
    };
    let included = tool_class_set(&input.requested_scope.included_tool_classes);
    let excluded = tool_class_set(&input.requested_scope.excluded_tool_classes);
    let has_explicit_includes = !included.is_empty();

    for tool_class in input.reachable_tool_classes {
        if excluded.contains(&tool_class) {
            let row = matrix.row(&tool_class);
            let unsupported = row
                .as_ref()
                .map(|r| r.safety_class == SafetyClass::UNSUPPORTED)
                .unwrap_or(true);
            if unsupported {
                result.unsupported_classes.push(tool_class.clone());
                let mut state = ToolClassReadiness {
                    tool_class: tool_class.clone(),
                    status: "excluded".to_string(),
                    excluded: true,
                    reason_code: "live_validation.unsupported_excluded".to_string(),
                    message: "Unsupported tool class is explicitly excluded from live validation."
                        .to_string(),
                    ..ToolClassReadiness::default()
                };
                if let Some(r) = &row {
                    state.matrix_row = Some(r.clone());
                }
                result.tool_classes.push(state);
                result.status = max_readiness(
                    &result.status,
                    &ReadinessStatus::from(ReadinessStatus::PARTIAL),
                );
                continue;
            }
            if let Some(r) = &row {
                result.tool_classes.push(ToolClassReadiness {
                    tool_class: tool_class.clone(),
                    status: "excluded".to_string(),
                    excluded: true,
                    matrix_row: Some(r.clone()),
                    reason_code: "live_validation.scope_excluded".to_string(),
                    message: "Tool class is outside the requested live-validation scope."
                        .to_string(),
                });
                result.status = max_readiness(
                    &result.status,
                    &ReadinessStatus::from(ReadinessStatus::PARTIAL),
                );
            }
            continue;
        }

        if has_explicit_includes && !included.contains(&tool_class) {
            let row = matrix.row(&tool_class);
            let Some(r) = row else {
                result.unsupported_classes.push(tool_class.clone());
                result.tool_classes.push(ToolClassReadiness {
                    tool_class: tool_class.clone(),
                    status: "unsupported".to_string(),
                    reason_code: "live_validation.matrix_row_missing".to_string(),
                    message:
                        "Unsupported tool class outside the included scope must be explicitly excluded."
                            .to_string(),
                    ..ToolClassReadiness::default()
                });
                result.status = ReadinessStatus::from(ReadinessStatus::BLOCKED);
                continue;
            };
            if r.safety_class == SafetyClass::UNSUPPORTED {
                result.unsupported_classes.push(tool_class.clone());
                result.tool_classes.push(ToolClassReadiness {
                    tool_class: tool_class.clone(),
                    status: "unsupported".to_string(),
                    matrix_row: Some(r.clone()),
                    reason_code: "live_validation.unsupported_tool_class".to_string(),
                    message:
                        "Unsupported tool class outside the included scope must be explicitly excluded."
                            .to_string(),
                    ..ToolClassReadiness::default()
                });
                result.status = ReadinessStatus::from(ReadinessStatus::BLOCKED);
                continue;
            }
            result.tool_classes.push(ToolClassReadiness {
                tool_class: tool_class.clone(),
                status: "excluded".to_string(),
                excluded: true,
                reason_code: "live_validation.scope_excluded".to_string(),
                message: "Tool class is outside the requested live-validation scope.".to_string(),
                ..ToolClassReadiness::default()
            });
            continue;
        }

        let row = matrix.row(&tool_class);
        let Some(r) = row else {
            result.unsupported_classes.push(tool_class.clone());
            result.tool_classes.push(ToolClassReadiness {
                tool_class: tool_class.clone(),
                status: "unsupported".to_string(),
                reason_code: "live_validation.matrix_row_missing".to_string(),
                message: "Tool class has no replay support matrix row.".to_string(),
                ..ToolClassReadiness::default()
            });
            result.status = ReadinessStatus::from(ReadinessStatus::BLOCKED);
            continue;
        };
        if r.safety_class == SafetyClass::UNSUPPORTED {
            result.unsupported_classes.push(tool_class.clone());
            result.tool_classes.push(ToolClassReadiness {
                tool_class: tool_class.clone(),
                status: "unsupported".to_string(),
                matrix_row: Some(r.clone()),
                reason_code: "live_validation.unsupported_tool_class".to_string(),
                message: "Tool class is unsupported for live validation.".to_string(),
                ..ToolClassReadiness::default()
            });
            result.status = ReadinessStatus::from(ReadinessStatus::BLOCKED);
            continue;
        }
        result.runnable_tool_classes.push(tool_class.clone());
        result.tool_classes.push(ToolClassReadiness {
            tool_class,
            status: "supported".to_string(),
            matrix_row: Some(r),
            ..ToolClassReadiness::default()
        });
    }
    result
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::matrix::Matrix;
    use crate::matrix::ToolClass;
    use crate::matrix::default_matrix_rows;
    use crate::types::SideEffectScope;

    fn matrix() -> Matrix {
        Matrix::new(default_matrix_rows()).expect("default matrix")
    }

    #[test]
    fn blocks_unsupported_classes() {
        let result = evaluate_candidate_readiness(
            &matrix(),
            CandidateReadinessInput {
                candidate_id: "candidate_1".to_string(),
                reachable_tool_classes: vec![
                    ToolClass::from(ToolClass::DAEMON_INSPECTION_READ),
                    ToolClass::from(ToolClass::MCP_TOOL_CALL),
                ],
                requested_scope: SideEffectScope {
                    included_tool_classes: vec![
                        ToolClass::from(ToolClass::DAEMON_INSPECTION_READ),
                        ToolClass::from(ToolClass::MCP_TOOL_CALL),
                    ],
                    ..SideEffectScope::default()
                },
            },
        );
        assert_eq!(
            result.status,
            ReadinessStatus::from(ReadinessStatus::BLOCKED)
        );
        assert_eq!(
            result.unsupported_classes,
            vec![ToolClass::from(ToolClass::MCP_TOOL_CALL)]
        );
    }

    #[test]
    fn allows_mixed_candidate_when_unsupported_excluded() {
        let result = evaluate_candidate_readiness(
            &matrix(),
            CandidateReadinessInput {
                candidate_id: "candidate_1".to_string(),
                reachable_tool_classes: vec![
                    ToolClass::from(ToolClass::DAEMON_INSPECTION_READ),
                    ToolClass::from(ToolClass::MCP_TOOL_CALL),
                ],
                requested_scope: SideEffectScope {
                    included_tool_classes: vec![ToolClass::from(ToolClass::DAEMON_INSPECTION_READ)],
                    excluded_tool_classes: vec![ToolClass::from(ToolClass::MCP_TOOL_CALL)],
                    ..SideEffectScope::default()
                },
            },
        );
        assert_eq!(
            result.status,
            ReadinessStatus::from(ReadinessStatus::PARTIAL)
        );
        assert_eq!(
            result.runnable_tool_classes,
            vec![ToolClass::from(ToolClass::DAEMON_INSPECTION_READ)]
        );
        let saw_excluded = result
            .tool_classes
            .iter()
            .any(|s| s.tool_class == ToolClass::MCP_TOOL_CALL && s.excluded);
        assert!(saw_excluded);
    }
}
