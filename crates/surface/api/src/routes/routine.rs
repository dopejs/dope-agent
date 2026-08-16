//! routines route family (port of daemon/internal/api/routine.go, Roadmap 66).
//!
//! Routes: GET/POST /v1/routines, POST /v1/routines/preview,
//! GET|PUT /v1/routines/{routine_id}, and POST
//! /v1/routines/{routine_id}/{pause|resume|cancel|repair}. Routines are
//! explicit configuration compiled onto the scheduler; preview is a dry-run
//! that references no existing routine. Error mapping preserves Go
//! writeRoutineError: not-found -> 404, invalid definition / cancelled -> 400,
//! scheduler compile/repair failures -> 500.

use axum::body::Bytes;
use axum::extract::{Path, State};
use axum::http::StatusCode;
use axum::routing::{get, post};
use axum::{Json, Router};
use serde::{Deserialize, Serialize};

use dope_routine as routine;

use crate::error::ApiError;
use crate::state::AppState;

use super::decode_json_required;

/// Route family router. `/v1/routines/preview` is registered as a static
/// route so it wins over the `{routine_id}` capture (the Go handler special-
/// cases the "preview" path segment).
#[must_use]
pub fn router() -> Router<AppState> {
    Router::new()
        .route("/v1/routines", get(list_routines).post(create_routine))
        .route("/v1/routines/preview", post(preview_routine))
        .route("/v1/routines/{routine_id}", get(get_routine).put(update_routine))
        .route("/v1/routines/{routine_id}/{action}", post(routine_action))
}

/// Go RoutineRequest — create, update, or preview payload.
#[derive(Debug, Default, Deserialize)]
#[serde(rename_all = "camelCase", default)]
struct RoutineRequest {
    definition: routine::Definition,
}

#[derive(Debug, Serialize)]
struct RoutineListResponse {
    items: Vec<routine::Routine>,
}

fn manager(state: &AppState) -> Result<&routine::Manager, ApiError> {
    state
        .routines
        .as_deref()
        .ok_or_else(|| ApiError::internal("routine manager is not configured"))
}

fn map_routine_error(err: routine::RoutineError) -> ApiError {
    let message = err.to_string();
    match err {
        routine::RoutineError::RoutineNotFound => ApiError::NotFound(message),
        routine::RoutineError::RoutineCancelled
        | routine::RoutineError::InvalidNameRequired
        | routine::RoutineError::InvalidGoalRequired
        | routine::RoutineError::InvalidCronExprRequired
        | routine::RoutineError::InvalidFireAtRequired
        | routine::RoutineError::InvalidTriggerKind => ApiError::BadRequest(message),
        routine::RoutineError::CompileSchedule(_) | routine::RoutineError::RepairSchedule(_) => {
            ApiError::Internal(message)
        }
    }
}

/// GET /v1/routines (Go handleRoutines GET branch).
async fn list_routines(
    State(state): State<AppState>,
) -> Result<Json<RoutineListResponse>, ApiError> {
    let manager = manager(&state)?;
    Ok(Json(RoutineListResponse { items: manager.list() }))
}

/// POST /v1/routines (Go handleRoutines POST branch) — 201.
async fn create_routine(
    State(state): State<AppState>,
    body: Bytes,
) -> Result<(StatusCode, Json<routine::Routine>), ApiError> {
    let request: RoutineRequest = decode_json_required(&body)?;
    let manager = manager(&state)?;
    let created = manager.create(request.definition).map_err(map_routine_error)?;
    Ok((StatusCode::CREATED, Json(created)))
}

/// POST /v1/routines/preview (Go handleRoutineRoutes preview branch) — a
/// dry-run compilation that touches no scheduler state.
async fn preview_routine(
    State(state): State<AppState>,
    body: Bytes,
) -> Result<Json<routine::Preview>, ApiError> {
    let request: RoutineRequest = decode_json_required(&body)?;
    let manager = manager(&state)?;
    let preview = manager.preview(&request.definition).map_err(map_routine_error)?;
    Ok(Json(preview))
}

/// GET /v1/routines/{routine_id} (Go handleRoutineRoutes get branch).
async fn get_routine(
    State(state): State<AppState>,
    Path(routine_id): Path<String>,
) -> Result<Json<routine::Routine>, ApiError> {
    let manager = manager(&state)?;
    manager
        .get(routine_id.trim())
        .map(Json)
        .ok_or_else(|| map_routine_error(routine::RoutineError::RoutineNotFound))
}

/// PUT /v1/routines/{routine_id} (Go handleRoutineRoutes update branch).
async fn update_routine(
    State(state): State<AppState>,
    Path(routine_id): Path<String>,
    body: Bytes,
) -> Result<Json<routine::Routine>, ApiError> {
    let request: RoutineRequest = decode_json_required(&body)?;
    let manager = manager(&state)?;
    let updated = manager
        .update(routine_id.trim(), request.definition)
        .map_err(map_routine_error)?;
    Ok(Json(updated))
}

/// POST /v1/routines/{routine_id}/{pause|resume|cancel|repair} (Go
/// handleRoutineRoutes action branch); unknown actions are 404 like the Go
/// http.NotFound fallthrough.
async fn routine_action(
    State(state): State<AppState>,
    Path((routine_id, action)): Path<(String, String)>,
) -> Result<Json<routine::Routine>, ApiError> {
    let manager = manager(&state)?;
    let routine_id = routine_id.trim();
    let result = match action.as_str() {
        "pause" => manager.pause(routine_id),
        "resume" => manager.resume(routine_id),
        "cancel" => manager.cancel(routine_id),
        "repair" => manager.repair(routine_id),
        _ => return Err(ApiError::NotFound("not found".to_string())),
    };
    result.map(Json).map_err(map_routine_error)
}

#[cfg(test)]
mod tests {
    use super::super::tests_support::{request_json, test_state};
    use axum::http::StatusCode;
    use std::sync::Arc;

    /// Scheduler fake mirroring the Go test scheduler: every call succeeds
    /// with a paused/resumed/cancelled schedule echo.
    struct FakeScheduler;

    impl dope_routine::Scheduler for FakeScheduler {
        fn create(
            &self,
            input: &dope_routine::CreateInput,
        ) -> Result<dope_routine::Schedule, String> {
            Ok(dope_routine::Schedule {
                schedule_id: "sched_fake_1".to_string(),
                trigger: input.trigger.clone(),
                target: input.target.clone(),
                retry_policy: input.retry_policy.clone(),
                ..dope_routine::Schedule::default()
            })
        }

        fn pause(&self, schedule_id: &str) -> Result<(dope_routine::Schedule, bool), String> {
            Ok((echo(schedule_id), true))
        }

        fn resume(&self, schedule_id: &str) -> Result<(dope_routine::Schedule, bool), String> {
            Ok((echo(schedule_id), true))
        }

        fn cancel(&self, schedule_id: &str) -> Result<(dope_routine::Schedule, bool), String> {
            Ok((echo(schedule_id), true))
        }

        fn get(&self, schedule_id: &str) -> Result<(dope_routine::Schedule, bool), String> {
            Ok((echo(schedule_id), true))
        }
    }

    fn echo(schedule_id: &str) -> dope_routine::Schedule {
        dope_routine::Schedule {
            schedule_id: schedule_id.to_string(),
            ..dope_routine::Schedule::default()
        }
    }

    fn state_with_manager() -> crate::state::AppState {
        let mut state = test_state();
        state.routines =
            Some(Arc::new(dope_routine::Manager::new("test", Box::new(FakeScheduler))));
        state
    }

    fn definition_body() -> serde_json::Value {
        serde_json::json!({
            "definition": {
                "name": "morning briefing",
                "trigger": { "kind": "cron", "cronExpr": "0 8 * * *" },
                "workflow": { "goal": "summarize the inbox" }
            }
        })
    }

    #[tokio::test]
    async fn preview_create_get_and_pause_routine() {
        let state = state_with_manager();
        let (status, preview) = request_json(
            state.clone(),
            "POST",
            "/v1/routines/preview",
            Some(definition_body()),
        )
        .await;
        assert_eq!(status, StatusCode::OK, "{preview}");

        let (status, created) =
            request_json(state.clone(), "POST", "/v1/routines", Some(definition_body())).await;
        assert_eq!(status, StatusCode::CREATED, "{created}");
        let routine_id = created["routineId"].as_str().expect("routineId").to_string();

        let (status, fetched) =
            request_json(state.clone(), "GET", &format!("/v1/routines/{routine_id}"), None).await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(fetched["routineId"], routine_id.as_str());

        let (status, paused) = request_json(
            state,
            "POST",
            &format!("/v1/routines/{routine_id}/pause"),
            None,
        )
        .await;
        assert_eq!(status, StatusCode::OK, "{paused}");
    }

    #[tokio::test]
    async fn invalid_definition_is_400_and_missing_routine_is_404() {
        let state = state_with_manager();
        let (status, body) = request_json(
            state.clone(),
            "POST",
            "/v1/routines",
            Some(serde_json::json!({
                "definition": {
                    "name": "",
                    "trigger": { "kind": "cron", "cronExpr": "0 8 * * *" },
                    "workflow": { "goal": "g" }
                }
            })),
        )
        .await;
        assert_eq!(status, StatusCode::BAD_REQUEST, "{body}");

        let (status, _) =
            request_json(state.clone(), "GET", "/v1/routines/routine_missing", None).await;
        assert_eq!(status, StatusCode::NOT_FOUND);

        let (status, _) = request_json(
            state,
            "POST",
            "/v1/routines/routine_missing/frobnicate",
            None,
        )
        .await;
        assert_eq!(status, StatusCode::NOT_FOUND);
    }
}
