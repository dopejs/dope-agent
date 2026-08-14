//! Port of daemon/internal/connectors/matrix/dedupe.go: durable dedupe on the
//! tenant + connector + homeserver + conversation + matrix event identity.

use std::collections::HashSet;

use parking_lot::Mutex;

use crate::types::InboundEvent;

/// Go `DedupeCache` (made thread-safe for the runtime's shared use).
#[derive(Default)]
pub struct DedupeCache {
    seen: Mutex<HashSet<String>>,
}

/// Go `NewDedupeCache`.
#[must_use]
pub fn new_dedupe_cache() -> DedupeCache {
    DedupeCache::default()
}

impl DedupeCache {
    /// Go `MarkDuplicate`: records the event's dedupe key and reports whether
    /// it was already seen.
    pub fn mark_duplicate(&self, event: &InboundEvent) -> bool {
        let key = dedupe_key(event);
        let mut seen = self.seen.lock();
        if seen.contains(&key) {
            return true;
        }
        seen.insert(key);
        false
    }
}

/// Go `DedupeKey`: the durable identity of a Matrix event — the sync batch
/// and transaction ids must NOT participate.
#[must_use]
pub fn dedupe_key(event: &InboundEvent) -> String {
    [
        event.tenant_id.trim(),
        event.connector_id.trim(),
        event.homeserver_id.trim(),
        event.conversation_id.trim(),
        event.matrix_event_id.trim(),
    ]
    .join("\u{0}")
}
