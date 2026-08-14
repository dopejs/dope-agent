//! Port of the daemon/internal/events envelope types (bus.go): the runtime-event
//! envelope, scope, resource, and subscription filter. The in-process Bus and the domain
//! emitters (agent_profiles, billing, connector_*, thread_*, evaluation_product, etc.) are
//! ported incrementally; this crate establishes the wire types shared by the store, API, and
//! fan-out layers.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

/// Event categories owned by no tenant (the events table is the only mixed table in the
/// inventory). Rows in these categories MUST keep tenant_id NULL.
pub const GLOBAL_CATEGORIES: [&str; 6] = [
    "mcp",
    "provider",
    "system",
    "daemon.migration",
    "connector_global",
    "capability_global",
];

/// Reports whether the given category is in the hard-coded global set.
#[must_use]
pub fn is_global_category(category: &str) -> bool {
    GLOBAL_CATEGORIES.contains(&category)
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Scope {
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub session_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub run_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub workflow_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub workflow_step_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub schedule_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub schedule_attempt_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub step_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub computer_use_session_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub computer_use_action_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub connector_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub capability_id: String,
}

#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Resource {
    pub kind: String,
    pub id: String,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Event {
    pub event_id: String,
    pub sequence: i64,
    /// Not serialized: carried out-of-band by the store/tenancy layer.
    #[serde(skip)]
    pub environment_scope: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    pub category: String,
    pub name: String,
    pub occurred_at: DateTime<Utc>,
    pub scope: Scope,
    pub resource: Resource,
    pub payload: serde_json::Map<String, serde_json::Value>,
}

impl Default for Event {
    fn default() -> Self {
        Event {
            event_id: String::new(),
            sequence: 0,
            environment_scope: String::new(),
            tenant_id: String::new(),
            category: String::new(),
            name: String::new(),
            occurred_at: Utc::now(),
            scope: Scope::default(),
            resource: Resource::default(),
            payload: serde_json::Map::new(),
        }
    }
}

/// Subscription filter on the bus.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Filter {
    pub environment_scope: String,
    pub category: String,
    pub run_id: String,
    pub session_id: String,
    pub schedule_id: String,
    pub schedule_attempt_id: String,
    pub resource_kind: String,
    pub cursor: i64,
    pub tenant_owned_tenant_id: String,
    pub include_global: bool,
}
