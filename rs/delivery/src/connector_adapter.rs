//! Port of `daemon/internal/delivery/connector_adapter.go`: the channel-connector delivery
//! adapter (the wave-7 channels port that `adapters.rs` deferred). The Go adapter registers
//! connector reply senders, persists the outbound connector message + delivery-boundary
//! ledger rows, and gates telegram/slack/matrix delivery on hosted-setup eligibility.
//!
//! context.Context disappears in the sync port: the Go ctx-derived tenant binding (the
//! `ResolveActiveTenantBinding` / `ResolveDefaultPersonalTenantID` resolve order) is an
//! explicit [`ConnectorAdapter::set_tenant`] value with the store's default personal tenant
//! as fallback. The store tenant-binding resolution itself is not ported (see the store
//! connectors module), so connector-message rows are written with the record's own
//! tenant_id (empty when unbound), exactly like the store crate's unbound-context path.

use std::collections::HashMap;
use std::sync::Arc;

use chrono::Utc;
use dope_imtypes::{DeliveryDirection, DeliveryStatus, MessageRecord, OutboundReply, SentReply};
use dope_store::SQLiteStore;
use dope_store::connectors::ConnectorDeliveryBoundaryRecord;
use parking_lot::Mutex;

use crate::{DeliveryAdapter, DeliveryOutcome, DeliveryTarget, SendResult, TargetKind};

/// Port of the Go `connectorReplySender` interface, with the optional
/// `connectorKindReporter` interface assertion folded in as a default method.
pub trait ConnectorReplySender: Send + Sync {
    /// Port of `SendReply` (the context parameter disappears in the sync port).
    fn send_reply(&self, reply: OutboundReply) -> Result<SentReply, String>;

    /// Port of the optional `connectorKindReporter` assertion: the default (empty string)
    /// means the sender does not report a connector kind.
    fn connector_kind(&self) -> &str {
        ""
    }
}

/// The channel-connector delivery adapter (port of `ConnectorAdapter`). Register it with the
/// delivery [`Manager`](crate::Manager) so connector-route targets are delivered through the
/// registered connector reply senders.
pub struct ConnectorAdapter {
    store: Arc<Mutex<SQLiteStore>>,
    /// Stand-in for the Go context tenant binding consulted by hosted-setup gating.
    tenant: Mutex<String>,
    senders: Mutex<HashMap<String, Arc<dyn ConnectorReplySender>>>,
    kinds: Mutex<HashMap<String, String>>,
}

impl ConnectorAdapter {
    #[must_use]
    pub fn new(store: Arc<Mutex<SQLiteStore>>) -> Self {
        ConnectorAdapter {
            store,
            tenant: Mutex::new(String::new()),
            senders: Mutex::new(HashMap::new()),
            kinds: Mutex::new(HashMap::new()),
        }
    }

    /// Sets the tenant id used for hosted-setup delivery-eligibility lookups (the sync port
    /// of the Go context tenant). When unset, the store's default personal tenant is used,
    /// then the empty unbound tenant — the Go resolve order.
    pub fn set_tenant(&self, tenant_id: &str) {
        *self.tenant.lock() = tenant_id.trim().to_string();
    }

    /// Port of `Register`: registers a reply sender under `connector_id`, deriving the
    /// connector kind from the sender's `ConnectorKind` when it reports one.
    pub fn register(&self, connector_id: &str, sender: Arc<dyn ConnectorReplySender>) {
        self.register_connector(connector_id, None, sender);
    }

    /// Port of `RegisterConnector`: registers a reply sender with an explicit kind.
    pub fn register_connector(
        &self,
        connector_id: &str,
        kind: Option<&str>,
        sender: Arc<dyn ConnectorReplySender>,
    ) {
        let connector_id = connector_id.trim();
        if connector_id.is_empty() {
            return;
        }
        let kind = match kind.map(str::trim).filter(|k| !k.is_empty()) {
            Some(k) => k.to_string(),
            None => sender.connector_kind().trim().to_string(),
        };
        self.senders.lock().insert(connector_id.to_string(), sender);
        if !kind.is_empty() {
            self.kinds.lock().insert(connector_id.to_string(), kind);
        }
    }

    /// Port of `senderFor`: the registered sender and its recorded kind.
    fn sender_for(&self, connector_id: &str) -> (Option<Arc<dyn ConnectorReplySender>>, String) {
        let connector_id = connector_id.trim();
        let sender = self.senders.lock().get(connector_id).cloned();
        let kind = self
            .kinds
            .lock()
            .get(connector_id)
            .cloned()
            .unwrap_or_default();
        (sender, kind)
    }

    /// Port of `requireConnectorDeliveryReady`: telegram/slack/matrix delivery is blocked
    /// until the connector's hosted setup is delivery eligible. The Go nil-manager /
    /// nil-store guards are not representable here (the store is required by construction).
    fn require_connector_delivery_ready(
        &self,
        connector_kind: &str,
        connector_id: &str,
    ) -> Result<(), String> {
        let kind = connector_kind.trim();
        if kind != "telegram" && kind != "slack" && kind != "matrix" {
            return Ok(());
        }
        let tenant_id = self.resolve_tenant();
        let store = self.store.lock();
        if kind == "slack" {
            return match store.get_slack_hosted_setup(&tenant_id, connector_id)? {
                Some(setup) if setup.terminal_state == "ready" && setup.delivery_eligible => Ok(()),
                _ => Err(format!(
                    "slack connector {connector_id} is not delivery eligible"
                )),
            };
        }
        if kind == "matrix" {
            return match store.get_matrix_hosted_setup(&tenant_id, connector_id)? {
                Some(setup) if setup.terminal_state == "ready" && setup.delivery_eligible => Ok(()),
                _ => Err(format!(
                    "matrix connector {connector_id} is not delivery eligible"
                )),
            };
        }
        match store.get_telegram_hosted_setup(&tenant_id, connector_id)? {
            Some(setup) if setup.hosted_ready && setup.delivery_eligible => Ok(()),
            _ => Err(format!(
                "telegram connector {connector_id} is not delivery eligible"
            )),
        }
    }

    /// Go tenant resolution: explicit binding first, then the store default personal tenant,
    /// then the empty unbound tenant.
    fn resolve_tenant(&self) -> String {
        let binding = self.tenant.lock().clone();
        if !binding.trim().is_empty() {
            return binding;
        }
        match self.store.lock().resolve_default_personal_tenant_id() {
            Ok(id) if !id.trim().is_empty() => id.trim().to_string(),
            _ => String::new(),
        }
    }
}

impl DeliveryAdapter for ConnectorAdapter {
    fn supports(&self, kind: TargetKind) -> bool {
        kind == TargetKind::ConnectorRoute
    }

    fn send(&self, target: DeliveryTarget, outcome: DeliveryOutcome) -> Result<SendResult, String> {
        let binding = target.connector_binding.as_ref().ok_or_else(|| {
            format!(
                "connector-backed delivery target {} is missing connector binding",
                target.target_id
            )
        })?;
        let connector_id = binding.connector_id.trim().to_string();
        if connector_id.is_empty() {
            return Err(format!(
                "connector-backed delivery target {} is missing connector binding",
                target.target_id
            ));
        }
        if binding.channel_id.trim().is_empty() {
            return Err(format!(
                "connector-backed delivery target {} is missing channel id",
                target.target_id
            ));
        }
        let (sender, connector_kind) = self.sender_for(&connector_id);
        let Some(sender) = sender else {
            return Err(format!(
                "connector {connector_id} is not available for delivery"
            ));
        };
        self.require_connector_delivery_ready(&connector_kind, &connector_id)?;

        let now = Utc::now();
        let mut record = MessageRecord {
            delivery_id: outcome.delivery_id.clone(),
            connector_id: connector_id.clone(),
            direction: DeliveryDirection::Outbound,
            run_id: outcome.run_id.clone(),
            channel_id: binding.channel_id.clone(),
            peer_id: binding.peer_id.clone(),
            thread_id: binding.thread_id.clone(),
            content: outcome.payload_preview.clone(),
            status: DeliveryStatus::Processing,
            response_to_delivery_id: outcome.delivery_id.clone(),
            background_delivery_id: outcome.delivery_id.clone(),
            delivery_boundary_kind: "background_delivery".to_string(),
            created_at: now,
            updated_at: now,
            ..MessageRecord::default()
        };
        self.store.lock().upsert_connector_message(&record)?;

        let sent = match sender.send_reply(OutboundReply {
            connector_id: connector_id.clone(),
            channel_id: binding.channel_id.clone(),
            content: outcome.payload_preview.clone(),
            ..OutboundReply::default()
        }) {
            Ok(sent) => sent,
            Err(err) => {
                record.updated_at = Utc::now();
                record.status = DeliveryStatus::Failed;
                record.error = err.clone();
                let _ = self.store.lock().upsert_connector_message(&record);
                return Err(err);
            }
        };

        record.updated_at = Utc::now();
        record.external_message_id = sent.external_message_id.clone();
        record.status = DeliveryStatus::Replied;
        let boundary_id = format!("boundary_{}", record.delivery_id);
        {
            let store = self.store.lock();
            store.upsert_connector_message(&record)?;
            let document = serde_json::to_string(&serde_json::json!({
                "boundaryId": boundary_id,
                "connectorId": record.connector_id,
                "foregroundReplyOutcomeId": outcome.delivery_id,
                "backgroundDeliveryId": record.delivery_id,
                "transportKind": TargetKind::ConnectorRoute.as_str(),
                "separationStatus": "separate_truths",
                "connectorMessageId": record.delivery_id,
                "externalMessageId": record.external_message_id,
            }))
            .map_err(|e| format!("marshal connector delivery boundary: {e}"))?;
            store.save_connector_delivery_boundary(&ConnectorDeliveryBoundaryRecord {
                boundary_id: boundary_id.clone(),
                connector_id: record.connector_id.clone(),
                foreground_reply_outcome_id: outcome.delivery_id.clone(),
                background_delivery_id: record.delivery_id.clone(),
                transport_kind: TargetKind::ConnectorRoute.as_str().to_string(),
                separation_status: "separate_truths".to_string(),
                created_at: record.updated_at,
                document,
                ..ConnectorDeliveryBoundaryRecord::default()
            })?;
        }

        Ok(SendResult {
            transport_kind: TargetKind::ConnectorRoute.as_str().to_string(),
            receipt_summary: "connector reply persisted".to_string(),
            connector_message_delivery_id: record.delivery_id,
            connector_delivery_boundary_id: boundary_id,
            separation_status: "separate_truths".to_string(),
        })
    }
}
