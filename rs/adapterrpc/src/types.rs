//! RPC envelope types, wire-compatible with the Go package
//! (`schemas/capability/integration-adapter/{request,response}.json`).

use serde::{Deserialize, Deserializer, Serialize, Serializer};
use serde_json::value::RawValue;

use crate::FailureKind;

/// The daemon's integration-adapter contract version. The daemon requires an exact match
/// with the adapter (FR-008, spec clarification Q3).
pub const CONTRACT_VERSION: &str = "1";

/// Outcome of an adapter operation (`"ok"` / `"failure"`).
///
/// Like the Go `Status string`, unknown wire values are preserved in [`Status::Other`]
/// instead of failing decode, so a non-conformant status is classified downstream exactly
/// as the Go daemon does (anything that is not `"failure"` takes the ok path).
#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub enum Status {
    Ok,
    Failure,
    /// Non-contract status string, preserved verbatim.
    Other(String),
}

impl Status {
    pub fn as_str(&self) -> &str {
        match self {
            Status::Ok => "ok",
            Status::Failure => "failure",
            Status::Other(s) => s,
        }
    }

    fn from_wire(s: &str) -> Self {
        match s {
            "ok" => Status::Ok,
            "failure" => Status::Failure,
            other => Status::Other(other.to_owned()),
        }
    }
}

impl std::fmt::Display for Status {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

impl Serialize for Status {
    fn serialize<S: Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        serializer.serialize_str(self.as_str())
    }
}

impl<'de> Deserialize<'de> for Status {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        let s = String::deserialize(deserializer)?;
        Ok(Status::from_wire(&s))
    }
}

/// Daemon -> adapter envelope for one Backend operation.
/// Mirrors `schemas/capability/integration-adapter/request.json`.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Request {
    pub request_id: String,
    pub contract_version: String,
    pub domain: String,
    pub operation: String,
    pub deadline_ms: i64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub resource: Option<Box<RawValue>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub credential: Option<Box<RawValue>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub payload: Option<Box<RawValue>>,
}

/// Adapter -> daemon envelope.
/// Mirrors `schemas/capability/integration-adapter/response.json`. It MUST NOT carry ledger,
/// idempotency, or side-effect-evidence state (FR-003).
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Response {
    pub request_id: String,
    pub contract_version: String,
    pub status: Status,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub failure_kind: Option<FailureKind>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub payload: Option<Box<RawValue>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub diagnostic: Option<Box<RawValue>>,
}
