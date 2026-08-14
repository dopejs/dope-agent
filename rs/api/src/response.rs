//! Response helpers.
//!
//! Port of the Go writeJSON helper (daemon/internal/api/server.go): a JSON
//! response with `Content-Type: application/json` and an explicit status.
//! `Json<T>` writes the payload bare (matching writeJSON); `success(data)`
//! wraps it in a `{data: ...}` envelope for route families that use one.

use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use serde::Serialize;

/// A JSON response with an explicit status code. Mirrors Go writeJSON: the
/// payload is serialized as-is with `Content-Type: application/json`.
#[derive(Debug, Clone, Copy, Default)]
pub struct Json<T>(pub T);

impl<T: Serialize> IntoResponse for Json<T> {
    fn into_response(self) -> Response {
        (StatusCode::OK, axum::Json(self.0)).into_response()
    }
}

/// Envelope for the success helper: `{ "data": <payload> }`.
#[derive(Debug, Serialize)]
pub struct DataEnvelope<T> {
    pub data: T,
}

/// Wraps a payload in a `{data: ...}` envelope (200 OK). Use this where the Go
/// surface wraps a response body; `Json<T>` for bare payloads.
#[must_use]
pub fn success<T: Serialize>(data: T) -> Json<DataEnvelope<T>> {
    Json(DataEnvelope { data })
}
