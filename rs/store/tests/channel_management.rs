//! Round-trip integration tests for the channel-management ledger DAOs ported from
//! `daemon/internal/store/channel_management.go` into `channel_management.rs`.
//! Ports of TestChannelManagementSupportEvidenceRetentionExpiresNormalInspection and
//! TestChannelManagementRouteReplyAndDeliveryOutcomesPersistWithRetention.

use std::collections::HashMap;

use chrono::{Duration, TimeZone, Utc};
use dope_connectors::RedactionStatus;
use dope_store::{
    BackgroundDeliveryOutcome, ForegroundReplyOutcome, ManagementState, RouteDecisionOutcome,
    RoutePolicy, RoutingDecision, SQLiteStore, SupportEvidenceBundle,
};

fn temp_dir(name: &str) -> String {
    let dir = std::env::temp_dir().join(format!("dope_store_{name}_{}", std::process::id()));
    let _ = std::fs::remove_dir_all(&dir);
    dir.to_string_lossy().to_string()
}

fn evidence(tenant_id: &str, connector_id: &str) -> HashMap<String, String> {
    let mut map = HashMap::new();
    map.insert("tenant".to_string(), tenant_id.to_string());
    map.insert("connector".to_string(), connector_id.to_string());
    map
}

#[test]
fn route_policy_saves_with_snapshot_and_round_trips() {
    let dir = temp_dir("channel_route_policy");
    let store = SQLiteStore::new(&dir).unwrap();
    let now = Utc.with_ymd_and_hms(2026, 5, 10, 10, 0, 0).unwrap();

    let mut policy = RoutePolicy {
        route_policy_id: "route_policy_active".to_string(),
        tenant_id: "ten_channels".to_string(),
        connector_id: "matrix-main".to_string(),
        eligible_senders: Vec::new(),
        eligible_conversations: Vec::new(),
        eligible_rooms: vec!["room_redacted".to_string()],
        eligible_channels: Vec::new(),
        invocation_gates: Vec::new(),
        background_delivery_eligible: false,
        validation_state: "valid".to_string(),
        reason_code: String::new(),
        validated_at: now,
        audit_event_id: "audit_route".to_string(),
        redaction_status: RedactionStatus::Redacted,
    };
    store.save_channel_route_policy(&policy).unwrap();
    // Upsert again through the ON CONFLICT path with a changed field.
    policy.background_delivery_eligible = true;
    policy.validated_at = now + Duration::minutes(1);
    store.save_channel_route_policy(&policy).unwrap();

    let got = store
        .get_channel_route_policy("ten_channels", "matrix-main")
        .unwrap()
        .expect("policy found");
    assert_eq!(got.route_policy_id, "route_policy_active");
    assert_eq!(got.validation_state, "valid");
    assert_eq!(got.eligible_rooms, vec!["room_redacted".to_string()]);
    assert!(got.background_delivery_eligible);

    assert!(store.get_channel_route_policy("ten_other", "matrix-main").unwrap().is_none());

    // Saving the same route_policy_id upserts its single snapshot row (Go writes
    // both tables, keyed by route_policy_id); a distinct policy id gets its own row.
    let conn = rusqlite::Connection::open(store.db_path()).unwrap();
    let snapshots: i64 = conn
        .query_row(
            "SELECT COUNT(*) FROM channel_route_policy_snapshots WHERE route_policy_id = ?1",
            [&"route_policy_active"],
            |row| row.get(0),
        )
        .unwrap();
    assert_eq!(snapshots, 1);
    store
        .save_channel_route_policy(&RoutePolicy {
            route_policy_id: "route_policy_second".to_string(),
            eligible_rooms: vec!["room_two".to_string()],
            validated_at: now,
            ..policy.clone()
        })
        .unwrap();
    let all_snapshots: i64 = conn
        .query_row("SELECT COUNT(*) FROM channel_route_policy_snapshots", [], |row| row.get(0))
        .unwrap();
    assert_eq!(all_snapshots, 2);
}

#[test]
fn routing_reply_and_delivery_outcomes_persist_with_retention() {
    let dir = temp_dir("channel_outcomes");
    let store = SQLiteStore::new(&dir).unwrap();
    let now = Utc.with_ymd_and_hms(2026, 5, 10, 10, 0, 0).unwrap();

    let decision = RoutingDecision {
        routing_decision_id: "route_active".to_string(),
        tenant_id: "ten_channels".to_string(),
        connector_id: "matrix-main".to_string(),
        connector_kind: "matrix".to_string(),
        outcome: RouteDecisionOutcome::Blocked,
        reason_code: "blocked_route".to_string(),
        occurred_at: now,
        safe_evidence: evidence("ten_channels", "matrix-main"),
        redaction_status: RedactionStatus::Redacted,
        retention_expires_at: now + Duration::days(90),
    };
    store.save_channel_routing_decision(&decision).unwrap();
    // Upsert again through the ON CONFLICT path with a changed field.
    let mut updated = decision.clone();
    updated.outcome = RouteDecisionOutcome::Accepted;
    updated.reason_code = "accepted".to_string();
    store.save_channel_routing_decision(&updated).unwrap();

    store
        .save_channel_routing_decision(&RoutingDecision {
            routing_decision_id: "route_expired".to_string(),
            occurred_at: now - Duration::days(100),
            retention_expires_at: now - Duration::hours(1),
            ..decision.clone()
        })
        .unwrap();

    store
        .save_channel_foreground_reply_outcome(&ForegroundReplyOutcome {
            reply_outcome_id: "reply_active".to_string(),
            tenant_id: "ten_channels".to_string(),
            connector_id: "matrix-main".to_string(),
            routing_decision_id: "route_active".to_string(),
            status: "failed".to_string(),
            reason_code: "provider_unavailable".to_string(),
            occurred_at: now,
            safe_evidence: evidence("ten_channels", "matrix-main"),
            redaction_status: RedactionStatus::Redacted,
            retention_expires_at: now + Duration::days(90),
        })
        .unwrap();
    store
        .save_channel_background_delivery_outcome(&BackgroundDeliveryOutcome {
            delivery_outcome_id: "delivery_active".to_string(),
            tenant_id: "ten_channels".to_string(),
            connector_id: "matrix-main".to_string(),
            delivery_target_id: "target_redacted".to_string(),
            status: "blocked".to_string(),
            reason_code: "connector_disabled".to_string(),
            occurred_at: now,
            safe_evidence: evidence("ten_channels", "matrix-main"),
            redaction_status: RedactionStatus::Redacted,
            retention_expires_at: now + Duration::days(90),
        })
        .unwrap();

    // Expired rows are filtered out; only the active one is listed.
    let decisions = store.list_channel_routing_decisions("ten_channels", "matrix-main", now).unwrap();
    assert_eq!(decisions.len(), 1);
    assert_eq!(decisions[0].routing_decision_id, "route_active");
    assert_eq!(decisions[0].outcome, RouteDecisionOutcome::Accepted);
    assert_eq!(decisions[0].safe_evidence.get("connector"), Some(&"matrix-main".to_string()));

    let replies = store.list_channel_foreground_reply_outcomes("ten_channels", "matrix-main", now).unwrap();
    assert_eq!(replies.len(), 1);
    assert_eq!(replies[0].routing_decision_id, "route_active");
    assert_eq!(replies[0].status, "failed");

    let deliveries = store.list_channel_background_delivery_outcomes("ten_channels", "matrix-main", now).unwrap();
    assert_eq!(deliveries.len(), 1);
    assert_eq!(deliveries[0].delivery_target_id, "target_redacted");

    // Cross-tenant list is empty.
    assert!(store.list_channel_routing_decisions("ten_other", "matrix-main", now).unwrap().is_empty());

    // List ordering: newest occurred_at first.
    store
        .save_channel_routing_decision(&RoutingDecision {
            routing_decision_id: "route_newer".to_string(),
            occurred_at: now + Duration::minutes(5),
            retention_expires_at: now + Duration::days(90),
            ..decision.clone()
        })
        .unwrap();
    let ordered = store.list_channel_routing_decisions("ten_channels", "matrix-main", now).unwrap();
    assert_eq!(ordered[0].routing_decision_id, "route_newer");
    assert_eq!(ordered[1].routing_decision_id, "route_active");
}

#[test]
fn support_evidence_retention_separates_latest_and_expired() {
    let dir = temp_dir("channel_support_evidence");
    let store = SQLiteStore::new(&dir).unwrap();
    let now = Utc.with_ymd_and_hms(2026, 5, 10, 10, 0, 0).unwrap();

    let expired = SupportEvidenceBundle {
        support_evidence_id: "support_expired".to_string(),
        tenant_id: "ten_channels".to_string(),
        connector_id: "discord-main".to_string(),
        generated_by_principal_id: String::new(),
        generated_at: now - Duration::days(100),
        current_state: ManagementState::Ready,
        state_transitions: vec!["ready".to_string()],
        diagnostic_refs: Vec::new(),
        repair_refs: Vec::new(),
        routing_decision_refs: Vec::new(),
        reply_outcome_refs: Vec::new(),
        delivery_outcome_refs: Vec::new(),
        audit_refs: Vec::new(),
        redactions: Vec::new(),
        retention_expires_at: now - Duration::hours(1),
        redaction_status: RedactionStatus::Redacted,
        safe_evidence: evidence("ten_channels", "discord-main"),
    };
    store.save_channel_support_evidence(&expired).unwrap();

    let mut active = expired.clone();
    active.support_evidence_id = "support_active".to_string();
    active.generated_at = now;
    active.retention_expires_at = now + Duration::days(90);
    store.save_channel_support_evidence(&active).unwrap();

    let latest = store
        .get_latest_channel_support_evidence("ten_channels", "discord-main", now)
        .unwrap()
        .expect("latest active bundle");
    assert_eq!(latest.support_evidence_id, "support_active");
    assert_eq!(latest.current_state, ManagementState::Ready);
    assert_eq!(latest.safe_evidence.get("connector"), Some(&"discord-main".to_string()));

    let expired_list = store
        .list_expired_channel_support_evidence("ten_channels", "discord-main", now)
        .unwrap();
    assert_eq!(expired_list.len(), 1);
    assert_eq!(expired_list[0].support_evidence_id, "support_expired");

    // Upserting an existing id refreshes the document only.
    let mut refreshed = active.clone();
    refreshed.current_state = ManagementState::Degraded;
    store.save_channel_support_evidence(&refreshed).unwrap();
    let latest = store
        .get_latest_channel_support_evidence("ten_channels", "discord-main", now)
        .unwrap()
        .expect("latest after refresh");
    assert_eq!(latest.current_state, ManagementState::Degraded);
}
