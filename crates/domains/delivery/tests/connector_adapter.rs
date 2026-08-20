//! Behavioral tests ported from `connector_adapter_test.go` and `matrix_adapter_test.go`: the
//! connector-backed delivery adapter (reply senders, connector message/boundary persistence,
//! hosted-setup eligibility gating) plus a manager-level emit_outcome round trip proving the
//! connector-route delivery path the wave-7 channels port deferred.

mod common;

use std::collections::HashMap;
use std::sync::Arc;

use chrono::Utc;
use kura_delivery::{
    ConnectorAdapter, ConnectorBinding, ConnectorReplySender, DeliveryAdapter, DeliveryOutcome,
    DeliveryPreference, DeliveryTarget, Manager, OutcomeInput, OutcomeStatus, PreferenceScopeKind,
    ResultClass, TargetKind, TargetStatus,
};
use kura_events::Bus;
use kura_imtypes::{DeliveryDirection, OutboundReply, SentReply};
use kura_runtime::{Run, RunStatus};
use kura_store::{
    MatrixHostedSetupRecord, SQLiteStore, SlackHostedSetupRecord, TelegramHostedSetupRecord,
};
use parking_lot::Mutex;

use common::store;

/// Go `fakeConnectorSender`: records outbound replies; the default external id is
/// "discord_reply_1" when none is supplied.
struct FakeConnectorSender {
    external_id: String,
    replies: Mutex<Vec<OutboundReply>>,
}

impl FakeConnectorSender {
    fn new(external_id: &str) -> Arc<Self> {
        Arc::new(FakeConnectorSender {
            external_id: external_id.to_string(),
            replies: Mutex::new(Vec::new()),
        })
    }

    fn replies(&self) -> Vec<OutboundReply> {
        self.replies.lock().clone()
    }
}

impl ConnectorReplySender for FakeConnectorSender {
    fn send_reply(&self, reply: OutboundReply) -> Result<SentReply, String> {
        self.replies.lock().push(reply);
        let external_id = if self.external_id.is_empty() {
            "discord_reply_1".to_string()
        } else {
            self.external_id.clone()
        };
        Ok(SentReply {
            external_message_id: external_id,
        })
    }
}

/// Go `kindedFakeConnectorSender`: a fake sender that reports a connector kind.
struct KindedFakeConnectorSender {
    inner: FakeConnectorSender,
    kind: String,
}

impl KindedFakeConnectorSender {
    fn new(external_id: &str, kind: &str) -> Arc<Self> {
        Arc::new(KindedFakeConnectorSender {
            inner: FakeConnectorSender {
                external_id: external_id.to_string(),
                replies: Mutex::new(Vec::new()),
            },
            kind: kind.to_string(),
        })
    }

    fn replies(&self) -> Vec<OutboundReply> {
        self.inner.replies()
    }
}

impl ConnectorReplySender for KindedFakeConnectorSender {
    fn send_reply(&self, reply: OutboundReply) -> Result<SentReply, String> {
        self.inner.send_reply(reply)
    }

    fn connector_kind(&self) -> &str {
        &self.kind
    }
}

/// The connector_messages table enforces a run_id foreign key (PRAGMA foreign_keys = ON), so
/// every delivery carrying a run id needs the run row first, exactly like the Go tests.
fn upsert_run(store: &Arc<Mutex<SQLiteStore>>, run_id: &str) {
    store
        .lock()
        .upsert_run(&Run {
            run_id: run_id.to_string(),
            entrypoint: "operator".to_string(),
            status: RunStatus::Completed,
            goal: "connector delivery".to_string(),
            ..Run::default()
        })
        .unwrap();
}

#[test]
fn connector_adapter_persists_outbound_transport_evidence() {
    let store = store("connector_discord");
    upsert_run(&store, "run_connector");

    let sender = FakeConnectorSender::new("");
    let adapter = ConnectorAdapter::new(Arc::clone(&store));
    adapter.register(
        "discord-main",
        Arc::clone(&sender) as Arc<dyn ConnectorReplySender>,
    );

    let result = adapter
        .send(
            DeliveryTarget {
                target_id: "discord-target".to_string(),
                target_kind: TargetKind::ConnectorRoute,
                connector_binding: Some(ConnectorBinding {
                    connector_id: "discord-main".to_string(),
                    channel_id: "channel-1".to_string(),
                    peer_id: "user-1".to_string(),
                    ..ConnectorBinding::default()
                }),
                ..DeliveryTarget::default()
            },
            DeliveryOutcome {
                delivery_id: "delivery_connector".to_string(),
                run_id: "run_connector".to_string(),
                payload_preview: "hello from delivery plane".to_string(),
                ..DeliveryOutcome::default()
            },
        )
        .expect("send should succeed");

    assert_eq!(sender.replies().len(), 1, "expected one outbound reply");
    assert_eq!(
        result.connector_message_delivery_id, "delivery_connector",
        "connector evidence should reuse the delivery id"
    );
    assert!(
        !result.connector_delivery_boundary_id.is_empty(),
        "expected connector delivery boundary evidence"
    );
    assert_eq!(result.separation_status, "separate_truths");

    let record = store
        .lock()
        .get_connector_message_by_external_id(
            "discord-main",
            DeliveryDirection::Outbound,
            "discord_reply_1",
        )
        .unwrap()
        .expect("stored connector message by external id");
    assert_eq!(record.response_to_delivery_id, "delivery_connector");
    assert_eq!(record.run_id, "run_connector");
    assert_eq!(record.background_delivery_id, "delivery_connector");
    assert_eq!(record.delivery_boundary_kind, "background_delivery");
}

#[test]
fn connector_adapter_supports_telegram_background_delivery_boundary() {
    let store = store("connector_telegram");
    let now = Utc::now();
    store
        .lock()
        .save_telegram_hosted_setup(&TelegramHostedSetupRecord {
            tenant_id: "ten_telegram_delivery".to_string(),
            connector_id: "telegram-main".to_string(),
            connector_kind: "telegram".to_string(),
            display_name: "Telegram Main".to_string(),
            status: "healthy".to_string(),
            terminal_state: "ready".to_string(),
            hosted_ready: true,
            credential_state: "valid".to_string(),
            allowment_state: "valid".to_string(),
            group_behavior: "mention_or_command_required".to_string(),
            delivery_eligible: true,
            reason_code: String::new(),
            redaction_status: "redacted".to_string(),
            created_at: now,
            updated_at: now,
            validated_at: Some(now),
            retention_expires_at: now + chrono::Duration::days(90),
            account_binding: None,
            allowments: Vec::new(),
        })
        .unwrap();
    upsert_run(&store, "run_telegram_delivery");

    let sender = KindedFakeConnectorSender::new("telegram_reply_1", "telegram");
    let adapter = ConnectorAdapter::new(Arc::clone(&store));
    adapter.set_tenant("ten_telegram_delivery");
    adapter.register(
        "telegram-main",
        Arc::clone(&sender) as Arc<dyn ConnectorReplySender>,
    );

    let result = adapter
        .send(
            DeliveryTarget {
                target_id: "telegram-target".to_string(),
                target_kind: TargetKind::ConnectorRoute,
                connector_binding: Some(ConnectorBinding {
                    connector_id: "telegram-main".to_string(),
                    channel_id: "telegram_chat_1".to_string(),
                    peer_id: "telegram_user_1".to_string(),
                    ..ConnectorBinding::default()
                }),
                ..DeliveryTarget::default()
            },
            DeliveryOutcome {
                delivery_id: "delivery_telegram".to_string(),
                run_id: "run_telegram_delivery".to_string(),
                payload_preview: "hello telegram".to_string(),
                ..DeliveryOutcome::default()
            },
        )
        .expect("send should succeed");

    let replies = sender.replies();
    assert_eq!(replies.len(), 1, "expected one Telegram outbound reply");
    assert_eq!(replies[0].connector_id, "telegram-main");
    assert_eq!(result.separation_status, "separate_truths");
    assert!(
        !result.connector_delivery_boundary_id.is_empty(),
        "expected Telegram delivery boundary"
    );

    let record = store
        .lock()
        .get_connector_message_by_external_id(
            "telegram-main",
            DeliveryDirection::Outbound,
            "telegram_reply_1",
        )
        .unwrap()
        .expect("stored telegram connector message");
    assert_eq!(record.background_delivery_id, "delivery_telegram");
    assert_eq!(record.delivery_boundary_kind, "background_delivery");
}

#[test]
fn connector_adapter_supports_slack_background_delivery_boundary() {
    let store = store("connector_slack");
    let now = Utc::now();
    store
        .lock()
        .save_slack_hosted_setup(&SlackHostedSetupRecord {
            tenant_id: "ten_slack_delivery".to_string(),
            connector_id: "slack-main".to_string(),
            connector_kind: "slack".to_string(),
            display_name: "Slack Main".to_string(),
            status: "healthy".to_string(),
            terminal_state: "ready".to_string(),
            oauth_state: "grant_valid".to_string(),
            route_policy_state: "valid".to_string(),
            delivery_eligible: true,
            workspace_binding_id: "slack_workspace_binding_delivery".to_string(),
            reason_code: String::new(),
            redaction_status: "redacted".to_string(),
            created_at: now,
            updated_at: now,
            validated_at: Some(now),
            retention_expires_at: now + chrono::Duration::days(90),
            workspace_binding: None,
            route_policy: None,
        })
        .unwrap();
    upsert_run(&store, "run_slack_delivery");

    let sender = KindedFakeConnectorSender::new("slack_reply_1", "slack");
    let adapter = ConnectorAdapter::new(Arc::clone(&store));
    adapter.set_tenant("ten_slack_delivery");
    adapter.register(
        "slack-main",
        Arc::clone(&sender) as Arc<dyn ConnectorReplySender>,
    );

    let result = adapter
        .send(
            DeliveryTarget {
                target_id: "slack-target".to_string(),
                target_kind: TargetKind::ConnectorRoute,
                connector_binding: Some(ConnectorBinding {
                    connector_id: "slack-main".to_string(),
                    channel_id: "slack_channel_1".to_string(),
                    peer_id: "slack_user_1".to_string(),
                    ..ConnectorBinding::default()
                }),
                ..DeliveryTarget::default()
            },
            DeliveryOutcome {
                delivery_id: "delivery_slack".to_string(),
                run_id: "run_slack_delivery".to_string(),
                payload_preview: "hello slack".to_string(),
                ..DeliveryOutcome::default()
            },
        )
        .expect("send should succeed");

    let replies = sender.replies();
    assert_eq!(replies.len(), 1, "expected one Slack outbound reply");
    assert_eq!(replies[0].connector_id, "slack-main");
    assert_eq!(result.separation_status, "separate_truths");
    assert!(
        !result.connector_delivery_boundary_id.is_empty(),
        "expected Slack delivery boundary"
    );

    let record = store
        .lock()
        .get_connector_message_by_external_id(
            "slack-main",
            DeliveryDirection::Outbound,
            "slack_reply_1",
        )
        .unwrap()
        .expect("stored slack connector message");
    assert_eq!(record.background_delivery_id, "delivery_slack");
    assert_eq!(record.delivery_boundary_kind, "background_delivery");
}

#[test]
fn connector_adapter_supports_matrix_background_delivery_boundary() {
    let store = store("connector_matrix");
    let now = Utc::now();
    store
        .lock()
        .save_matrix_hosted_setup(&MatrixHostedSetupRecord {
            tenant_id: "ten_matrix_delivery".to_string(),
            connector_id: "matrix-main".to_string(),
            connector_kind: "matrix".to_string(),
            display_name: "Matrix Main".to_string(),
            status: "healthy".to_string(),
            terminal_state: "ready".to_string(),
            bot_credential_state: "valid".to_string(),
            homeserver_state: "reachable".to_string(),
            route_policy_state: "valid".to_string(),
            delivery_eligible: true,
            homeserver_binding_id: "matrix_hs_delivery".to_string(),
            reason_code: String::new(),
            redaction_status: "redacted".to_string(),
            created_at: now,
            updated_at: now,
            validated_at: Some(now),
            retention_expires_at: now + chrono::Duration::days(90),
            homeserver_binding: None,
            route_policy: None,
        })
        .unwrap();
    upsert_run(&store, "run_matrix_delivery");

    let sender = KindedFakeConnectorSender::new("matrix_reply_1", "matrix");
    let adapter = ConnectorAdapter::new(Arc::clone(&store));
    adapter.set_tenant("ten_matrix_delivery");
    adapter.register(
        "matrix-main",
        Arc::clone(&sender) as Arc<dyn ConnectorReplySender>,
    );

    let result = adapter
        .send(
            DeliveryTarget {
                target_id: "matrix-target".to_string(),
                target_kind: TargetKind::ConnectorRoute,
                connector_binding: Some(ConnectorBinding {
                    connector_id: "matrix-main".to_string(),
                    channel_id: "!room:example.org".to_string(),
                    peer_id: "@alice:example.org".to_string(),
                    ..ConnectorBinding::default()
                }),
                ..DeliveryTarget::default()
            },
            DeliveryOutcome {
                delivery_id: "delivery_matrix".to_string(),
                run_id: "run_matrix_delivery".to_string(),
                payload_preview: "hello matrix".to_string(),
                ..DeliveryOutcome::default()
            },
        )
        .expect("send should succeed");

    let replies = sender.replies();
    assert_eq!(replies.len(), 1, "expected one Matrix outbound reply");
    assert_eq!(replies[0].connector_id, "matrix-main");
    assert_eq!(result.separation_status, "separate_truths");
    assert!(
        !result.connector_delivery_boundary_id.is_empty(),
        "expected Matrix delivery boundary"
    );

    let record = store
        .lock()
        .get_connector_message_by_external_id(
            "matrix-main",
            DeliveryDirection::Outbound,
            "matrix_reply_1",
        )
        .unwrap()
        .expect("stored matrix connector message");
    assert_eq!(record.background_delivery_id, "delivery_matrix");
    assert_eq!(record.delivery_boundary_kind, "background_delivery");
}

#[test]
fn connector_adapter_blocks_slack_delivery_until_hosted_setup_is_ready() {
    let store = store("connector_slack_blocked");
    let sender = FakeConnectorSender::new("slack_reply_1");
    let adapter = ConnectorAdapter::new(Arc::clone(&store));
    adapter.register_connector(
        "slack-main",
        Some("slack"),
        Arc::clone(&sender) as Arc<dyn ConnectorReplySender>,
    );

    let err = adapter
        .send(
            DeliveryTarget {
                target_id: "slack-target".to_string(),
                target_kind: TargetKind::ConnectorRoute,
                connector_binding: Some(ConnectorBinding {
                    connector_id: "slack-main".to_string(),
                    channel_id: "slack_channel_1".to_string(),
                    ..ConnectorBinding::default()
                }),
                ..DeliveryTarget::default()
            },
            DeliveryOutcome {
                delivery_id: "delivery_slack_blocked".to_string(),
                run_id: "run_slack_delivery".to_string(),
                payload_preview: "hello slack".to_string(),
                ..DeliveryOutcome::default()
            },
        )
        .expect_err("expected Slack delivery to be blocked before hosted setup is ready");
    assert!(
        err.contains("not delivery eligible"),
        "unexpected error: {err}"
    );
    assert_eq!(
        sender.replies().len(),
        0,
        "blocked Slack delivery should not send replies"
    );
}

#[test]
fn connector_adapter_blocks_telegram_delivery_until_hosted_setup_is_ready() {
    let store = store("connector_telegram_blocked");
    let sender = FakeConnectorSender::new("telegram_reply_1");
    let adapter = ConnectorAdapter::new(Arc::clone(&store));
    adapter.register_connector(
        "telegram-main",
        Some("telegram"),
        Arc::clone(&sender) as Arc<dyn ConnectorReplySender>,
    );

    let err = adapter
        .send(
            DeliveryTarget {
                target_id: "telegram-target".to_string(),
                target_kind: TargetKind::ConnectorRoute,
                connector_binding: Some(ConnectorBinding {
                    connector_id: "telegram-main".to_string(),
                    channel_id: "telegram_chat_1".to_string(),
                    ..ConnectorBinding::default()
                }),
                ..DeliveryTarget::default()
            },
            DeliveryOutcome {
                delivery_id: "delivery_telegram_blocked".to_string(),
                run_id: "run_telegram_delivery".to_string(),
                payload_preview: "hello telegram".to_string(),
                ..DeliveryOutcome::default()
            },
        )
        .expect_err("expected Telegram delivery to be blocked before hosted setup is ready");
    assert!(
        err.contains("not delivery eligible"),
        "unexpected error: {err}"
    );
    assert_eq!(
        sender.replies().len(),
        0,
        "blocked Telegram delivery should not send replies"
    );
}

#[test]
fn connector_adapter_blocks_matrix_delivery_until_hosted_setup_is_ready() {
    let store = store("connector_matrix_blocked");
    let sender = FakeConnectorSender::new("matrix_reply_1");
    let adapter = ConnectorAdapter::new(Arc::clone(&store));
    adapter.register_connector(
        "matrix-main",
        Some("matrix"),
        Arc::clone(&sender) as Arc<dyn ConnectorReplySender>,
    );

    let err = adapter
        .send(
            DeliveryTarget {
                target_id: "matrix-target".to_string(),
                target_kind: TargetKind::ConnectorRoute,
                connector_binding: Some(ConnectorBinding {
                    connector_id: "matrix-main".to_string(),
                    channel_id: "!room:example.org".to_string(),
                    ..ConnectorBinding::default()
                }),
                ..DeliveryTarget::default()
            },
            DeliveryOutcome {
                delivery_id: "delivery_matrix_blocked".to_string(),
                run_id: "run_matrix_delivery".to_string(),
                payload_preview: "hello matrix".to_string(),
                ..DeliveryOutcome::default()
            },
        )
        .expect_err("expected Matrix delivery to be blocked before hosted setup is ready");
    assert!(
        err.contains("not delivery eligible"),
        "unexpected error: {err}"
    );
    assert_eq!(
        sender.replies().len(),
        0,
        "blocked Matrix delivery should not send replies"
    );
}

#[test]
fn connector_adapter_emits_delivery_outcome_through_manager() {
    let store = store("connector_manager");
    upsert_run(&store, "run_manager_delivery");

    let sender = FakeConnectorSender::new("manager_reply_1");
    let connector = ConnectorAdapter::new(Arc::clone(&store));
    connector.register(
        "discord-main",
        Arc::clone(&sender) as Arc<dyn ConnectorReplySender>,
    );
    let adapter: Arc<dyn DeliveryAdapter> = Arc::new(connector);
    let manager = Manager::new("test", Bus::new(), Arc::clone(&store), vec![adapter]);

    manager
        .create_target(DeliveryTarget {
            target_id: "discord-target".to_string(),
            display_name: "Discord Main".to_string(),
            environment_scope: "test".to_string(),
            target_kind: TargetKind::ConnectorRoute,
            status: TargetStatus::Active,
            connector_binding: Some(ConnectorBinding {
                connector_id: "discord-main".to_string(),
                channel_id: "channel-1".to_string(),
                peer_id: "user-1".to_string(),
                ..ConnectorBinding::default()
            }),
            ..DeliveryTarget::default()
        })
        .unwrap();

    let mut by_class = HashMap::new();
    by_class.insert(ResultClass::Failure, "discord-target".to_string());
    manager
        .upsert_preference(DeliveryPreference {
            preference_id: "pref-connector".to_string(),
            environment_scope: "test".to_string(),
            scope_kind: PreferenceScopeKind::UserDefault,
            preferred_targets_by_class: by_class,
            ..DeliveryPreference::default()
        })
        .unwrap();

    let outcome = manager
        .emit_outcome(OutcomeInput {
            source_kind: "run".to_string(),
            source_id: "run_manager_delivery".to_string(),
            run_id: "run_manager_delivery".to_string(),
            result_class: ResultClass::Failure,
            payload_preview: "hello from manager".to_string(),
            ..OutcomeInput::default()
        })
        .unwrap();

    assert_eq!(
        outcome.status,
        OutcomeStatus::Delivered,
        "expected immediate connector delivery through the manager: {outcome:?}"
    );
    assert_eq!(outcome.chosen_target_id, "discord-target");
    assert_eq!(
        outcome.attempts.len(),
        1,
        "expected one delivery attempt: {outcome:?}"
    );
    assert_eq!(outcome.attempts[0].transport_kind, "connector_route");
    let replies = sender.replies();
    assert_eq!(replies.len(), 1, "expected one connector reply");
    assert_eq!(replies[0].connector_id, "discord-main");
    assert_eq!(replies[0].content, "hello from manager");
    let record = store
        .lock()
        .get_connector_message_by_external_id(
            "discord-main",
            DeliveryDirection::Outbound,
            "manager_reply_1",
        )
        .unwrap()
        .expect("stored connector message");
    assert_eq!(record.background_delivery_id, outcome.delivery_id);
    assert_eq!(record.response_to_delivery_id, outcome.delivery_id);
}
