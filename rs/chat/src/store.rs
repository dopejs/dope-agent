//! The store surface the chat service depends on.
//!
//! Go's `Service` holds a `*store.SQLiteStore` and calls a fixed set of CRUD
//! methods (dispatch persistence, event append, setup-session lookup, profile
//! selection, binding resolution, continuity turns/previews, handoff links).
//! The `dope-store` crate has ported only the first two so far, so this
//! module declares the full surface as a `ChatStore` trait (with the Go
//! query/parameter structs) and implements it for `dope_store::SQLiteStore`:
//! the ported methods delegate to SQLite, and the not-yet-ported methods fail
//! with [`ChatError::StoreMethodDeferred`] instead of silently degrading.
//! Tests exercise the whole pipeline against an in-memory fake store.

use chrono::{DateTime, Utc};
use dope_bindings::{CapabilityDecision, EffectiveBindingSelection, RuntimeBindingEvidence};
use dope_events::Event;
use dope_llm::Dispatch;
use dope_profiles::{ActiveSelection, AgentProfile, RuntimeProjection};
use dope_setupwizard::SetupSession;
use dope_threads::{
    ContinuityPreview, ContinuityPreviewItem, ContinuityTurn, HandoffLink,
    HandoffSourceReference, Thread,
};

/// Go `store.ContinuityLookupQuery`. A `now` of `None` maps to Go's zero
/// `time.Time` (the store substitutes the current time); `limit <= 0` lets
/// the store apply its default (`DefaultContinuityMaxPriorTurns + 64`).
#[derive(Debug, Clone, Default, PartialEq)]
pub struct ContinuityLookupQuery {
    pub tenant_id: String,
    pub thread_id: String,
    pub session_segment_id: String,
    pub limit: i64,
    pub now: Option<DateTime<Utc>>,
}

/// Go `store.BindingResolutionParams` (binding_resolution.go).
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct BindingResolutionParams {
    pub tenant_id: String,
    pub channel_scope_ref: String,
    pub account_scope_ref: String,
    pub tenant_default_profile_id: String,
    pub tenant_default_profile_version_id: String,
}

/// The store methods `daemon/internal/chat/service.go` calls on
/// `store.SQLiteStore`, in Go signature order.
pub trait ChatStore: Send + Sync {
    // -- llm dispatch CRUD (ported in dope-store) -------------------------
    fn upsert_llm_dispatch(&self, dispatch: &Dispatch) -> Result<(), String>;
    // -- event ledger (ported in dope-store) ------------------------------
    fn append_event(&self, event: &Event) -> Result<Event, String>;
    // -- setup wizard (not yet ported to dope-store) ----------------------
    fn list_setup_sessions(&self, tenant_id: &str) -> Result<Vec<SetupSession>, String>;
    // -- profile projection (not yet ported) ------------------------------
    fn active_agent_profile_selection(
        &self,
        tenant_id: &str,
    ) -> Result<Option<(AgentProfile, ActiveSelection)>, String>;
    fn record_runtime_profile_projection(
        &self,
        projection: &RuntimeProjection,
    ) -> Result<RuntimeProjection, String>;
    // -- binding resolution + evidence (not yet ported) -------------------
    fn resolve_binding_selection(
        &self,
        params: &BindingResolutionParams,
    ) -> Result<EffectiveBindingSelection, String>;
    fn effective_capability_visibility(
        &self,
        tenant_id: &str,
        profile_id: &str,
        workspace_id: &str,
        capability_id: &str,
    ) -> Result<CapabilityDecision, String>;
    fn record_runtime_binding_evidence(
        &self,
        evidence: &RuntimeBindingEvidence,
    ) -> Result<RuntimeBindingEvidence, String>;
    // -- thread/continuity/handoff (not yet ported) -----------------------
    fn get_thread_for_tenant(
        &self,
        tenant_id: &str,
        thread_id: &str,
    ) -> Result<Option<Thread>, String>;
    fn list_continuity_turns(
        &self,
        query: &ContinuityLookupQuery,
    ) -> Result<Vec<ContinuityTurn>, String>;
    fn list_continuity_turns_outside_session_segment(
        &self,
        query: &ContinuityLookupQuery,
    ) -> Result<Vec<ContinuityTurn>, String>;
    fn list_handoff_links_for_thread(
        &self,
        tenant_id: &str,
        thread_id: &str,
        limit: i64,
    ) -> Result<Vec<HandoffLink>, String>;
    fn list_handoff_source_references_for_link(
        &self,
        tenant_id: &str,
        link_id: &str,
    ) -> Result<Vec<HandoffSourceReference>, String>;
    fn save_continuity_turn(
        &self,
        turn: &ContinuityTurn,
    ) -> Result<ContinuityTurn, String>;
    fn mark_handoff_source_references_consumed(
        &self,
        tenant_id: &str,
        link_id: &str,
        response_turn_id: &str,
        now: DateTime<Utc>,
    ) -> Result<(), String>;
    fn save_continuity_preview(
        &self,
        preview: &ContinuityPreview,
        items: &[ContinuityPreviewItem],
    ) -> Result<ContinuityPreview, String>;
}

/// Adapter for the `dope-store` SQLite store. The dispatch-persistence and
/// event-append methods delegate to SQLite; every other method is not ported
/// to `dope-store` yet and fails with a deferred error (see the module
/// documentation).
impl ChatStore for dope_store::SQLiteStore {
    fn upsert_llm_dispatch(&self, dispatch: &Dispatch) -> Result<(), String> {
        dope_store::SQLiteStore::upsert_llm_dispatch(self, dispatch)
    }
    fn append_event(&self, event: &Event) -> Result<Event, String> {
        dope_store::SQLiteStore::append_event(self, event)
    }
    fn list_setup_sessions(&self, _tenant_id: &str) -> Result<Vec<SetupSession>, String> {
        Err(deferred("list_setup_sessions"))
    }
    fn active_agent_profile_selection(
        &self,
        _tenant_id: &str,
    ) -> Result<Option<(AgentProfile, ActiveSelection)>, String> {
        Err(deferred("active_agent_profile_selection"))
    }
    fn record_runtime_profile_projection(
        &self,
        _projection: &RuntimeProjection,
    ) -> Result<RuntimeProjection, String> {
        Err(deferred("record_runtime_profile_projection"))
    }
    fn resolve_binding_selection(
        &self,
        _params: &BindingResolutionParams,
    ) -> Result<EffectiveBindingSelection, String> {
        Err(deferred("resolve_binding_selection"))
    }
    fn effective_capability_visibility(
        &self,
        _tenant_id: &str,
        _profile_id: &str,
        _workspace_id: &str,
        _capability_id: &str,
    ) -> Result<CapabilityDecision, String> {
        Err(deferred("effective_capability_visibility"))
    }
    fn record_runtime_binding_evidence(
        &self,
        _evidence: &RuntimeBindingEvidence,
    ) -> Result<RuntimeBindingEvidence, String> {
        Err(deferred("record_runtime_binding_evidence"))
    }
    fn get_thread_for_tenant(
        &self,
        _tenant_id: &str,
        _thread_id: &str,
    ) -> Result<Option<Thread>, String> {
        Err(deferred("get_thread_for_tenant"))
    }
    fn list_continuity_turns(
        &self,
        _query: &ContinuityLookupQuery,
    ) -> Result<Vec<ContinuityTurn>, String> {
        Err(deferred("list_continuity_turns"))
    }
    fn list_continuity_turns_outside_session_segment(
        &self,
        _query: &ContinuityLookupQuery,
    ) -> Result<Vec<ContinuityTurn>, String> {
        Err(deferred("list_continuity_turns_outside_session_segment"))
    }
    fn list_handoff_links_for_thread(
        &self,
        _tenant_id: &str,
        _thread_id: &str,
        _limit: i64,
    ) -> Result<Vec<HandoffLink>, String> {
        Err(deferred("list_handoff_links_for_thread"))
    }
    fn list_handoff_source_references_for_link(
        &self,
        _tenant_id: &str,
        _link_id: &str,
    ) -> Result<Vec<HandoffSourceReference>, String> {
        Err(deferred("list_handoff_source_references_for_link"))
    }
    fn save_continuity_turn(
        &self,
        _turn: &ContinuityTurn,
    ) -> Result<ContinuityTurn, String> {
        Err(deferred("save_continuity_turn"))
    }
    fn mark_handoff_source_references_consumed(
        &self,
        _tenant_id: &str,
        _link_id: &str,
        _response_turn_id: &str,
        _now: DateTime<Utc>,
    ) -> Result<(), String> {
        Err(deferred("mark_handoff_source_references_consumed"))
    }
    fn save_continuity_preview(
        &self,
        _preview: &ContinuityPreview,
        _items: &[ContinuityPreviewItem],
    ) -> Result<ContinuityPreview, String> {
        Err(deferred("save_continuity_preview"))
    }
}

fn deferred(name: &str) -> String {
    format!("chat store method not ported to dope-store: {name}")
}
