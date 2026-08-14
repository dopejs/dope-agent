//! Port of `daemon/internal/delivery/adapters.go`: the delivery adapter seam.
//!
//! Go declares one interface:
//!
//! ```go
//! type Adapter interface {
//!     Supports(kind TargetKind) bool
//!     Send(context.Context, DeliveryTarget, DeliveryOutcome) (SendResult, error)
//! }
//! ```
//!
//! The Rust port keeps the same two-method shape. context.Context disappears (the manager is
//! synchronous) and Go's (SendResult, error) pair becomes Result<SendResult, String>; the
//! error message is what the manager stores as the attempt/outcome failure reason, so the
//! String is the faithful carrier.
//!
//! ## Channel-specific adapters (deferred)
//!
//! The wave-7 channel adapters — ConnectorAdapter (connector reply senders, connector
//! message/boundary persistence) and the matrix/telegram/slack hosted-setup delivery-eligibility
//! gating in connector_adapter.go — depend on the channel store domains (connector messages,
//! delivery boundaries, hosted setups, channel enablement state) that have not been ported to
//! the Rust store crate yet. Their bodies are DEFERRED. The manager treats a connector-route
//! target as having no registered adapter (see adapter_unavailable failures) until those
//! adapters land. The store interactions those adapters relied on are represented by
//! [ChannelDeliveryHooks] below, whose default implementations are no-ops pending the
//! channels port.

use crate::{DeliveryOutcome, DeliveryTarget, SendResult, TargetKind};

/// The transport adapter seam (port of Adapter).
///
/// Implementations must be shareable across the manager's retry/window threads.
pub trait DeliveryAdapter: Send + Sync {
    /// Reports whether this adapter can deliver to targets of the given kind.
    fn supports(&self, kind: TargetKind) -> bool;

    /// Sends one delivery. The target/outcome are passed by value (mirroring Go's value
    /// semantics: the manager keeps its own copy and applies the returned transport evidence).
    ///
    /// Go adapters may return a partial SendResult alongside an error (e.g. only the
    /// transport kind filled in); the Rust trait models the failure as Err(String) and the
    /// manager preserves the pre-send transport kind on the attempt in that case, which matches
    /// the behavior of the ported Go adapters (test sink returns an empty partial result).
    fn send(&self, target: DeliveryTarget, outcome: DeliveryOutcome) -> Result<SendResult, String>;
}

/// Store-domain hooks the Go manager calls for channel and thread subsystems whose Rust store
/// CRUD has not been ported yet.
///
/// - connectorDeliveryDisabled: reads channel-connector enablement state
///   (store.GetChannelConnectorEnablementState).
/// - recordChannelBackgroundDeliveryOutcome: persists channel-management background delivery
///   evidence (store.SaveChannelBackgroundDeliveryOutcome).
/// - recordThreadDeliveryProjection: persists a thread runtime projection for background
///   deliveries (store.SaveThreadRuntimeProjectionForRun) and appends its projection event.
///
/// The default implementations are no-ops: connector targets are treated as enabled, and no
/// channel/thread evidence is recorded, until the wave-7 channel store domains are ported. The
/// manager only invokes the hooks for connector-route targets (and, for the thread projection,
/// outcomes carrying a run id), mirroring the Go guards exactly.
pub trait ChannelDeliveryHooks: Send + Sync {
    /// Returns Ok(true) when the connector's channel delivery is disabled.
    fn connector_delivery_disabled(&self, _connector_id: &str) -> Result<bool, String> {
        Ok(false)
    }

    /// Records channel-management background delivery evidence for a connector-route target.
    fn record_background_delivery_outcome(
        &self,
        _outcome: &DeliveryOutcome,
        _target: &DeliveryTarget,
        _reason_code: &str,
    ) -> Result<(), String> {
        Ok(())
    }

    /// Records a thread runtime projection for the outcome's run.
    fn record_thread_delivery_projection(
        &self,
        _outcome: &DeliveryOutcome,
        _reason_code: &str,
    ) -> Result<(), String> {
        Ok(())
    }
}
