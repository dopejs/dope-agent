//! Conformance harness: verifies an adapter against the contract (FR-009). Mirrors
//! `conformance.go`.

use crate::client::Client;
use crate::error::Error;

/// The representative operation set every integration adapter must satisfy to be
/// conformant. It mirrors the calendar and mail Backend interfaces.
pub const CONFORMANCE_OPS: &[(&str, &str)] = &[
    ("calendar", "ProjectAccount"),
    ("calendar", "ListEvents"),
    ("calendar", "GetEvent"),
    ("calendar", "BusyFree"),
    ("calendar", "CreateEvent"),
    ("calendar", "UpdateEvent"),
    ("calendar", "CancelEvent"),
    ("mail", "ProjectAccount"),
    ("mail", "ListThreads"),
    ("mail", "GetThread"),
    ("mail", "GetMessage"),
    ("mail", "ListDrafts"),
    ("mail", "GetDraft"),
    ("mail", "CreateDraft"),
    ("mail", "UpdateDraft"),
    ("mail", "SendMessage"),
    ("mail", "SendDraft"),
    ("mail", "ReplyMessage"),
    ("mail", "ForwardMessage"),
    ("mail", "ResolveAttachments"),
];

/// Outcome for one operation.
#[derive(Debug)]
pub struct ConformanceResult {
    pub domain: String,
    pub operation: String,
    pub ok: bool,
    pub err: Option<Error>,
}

/// Result of running an adapter against the contract.
#[derive(Debug, Default)]
pub struct ConformanceReport {
    pub ready_err: Option<Error>,
    pub results: Vec<ConformanceResult>,
}

impl ConformanceReport {
    /// Whether the adapter satisfied the readiness handshake and every operation.
    pub fn passed(&self) -> bool {
        if self.ready_err.is_some() || self.results.is_empty() {
            return false;
        }
        self.results.iter().all(|r| r.ok)
    }
}

/// Verify an adapter (reached via `client`) against the contract: perform the version
/// readiness handshake, then dispatch every contract operation and record whether each
/// returned a valid, contract-conformant response. A version mismatch, transport break, or
/// malformed response causes the relevant step to fail (FR-009). It does not mutate real
/// state — the adapter under test is expected to be the reference skeleton or a provider
/// adapter pointed at a safe target.
pub fn run_conformance(client: &Client) -> ConformanceReport {
    let mut report = ConformanceReport::default();
    if let Err(err) = client.ready() {
        report.ready_err = Some(err);
        return report;
    }
    for (domain, operation) in CONFORMANCE_OPS {
        let resource = serde_json::json!({});
        let err = client
            .dispatch::<serde_json::Value, serde_json::Value, serde_json::Value>(
                domain,
                operation,
                Some(&resource),
                None,
                None,
            )
            .err();
        report.results.push(ConformanceResult {
            domain: (*domain).to_owned(),
            operation: (*operation).to_owned(),
            ok: err.is_none(),
            err,
        });
    }
    report
}
