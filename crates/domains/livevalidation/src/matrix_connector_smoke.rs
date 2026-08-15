//! Matrix connector smoke evidence (port of `matrix_connector_smoke.go`).

use std::collections::BTreeMap;

use chrono::DateTime;
use chrono::Duration;
use chrono::Utc;
use serde::Deserialize;
use serde::Serialize;

/// Input to [`build_matrix_connector_smoke_evidence`].
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MatrixConnectorSmokeInput {
    pub tenant_id: String,
    pub connector_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub homeserver_binding_id: String,
    pub owner: String,
    pub reason: String,
    pub safe_live_available: bool,
    pub now: DateTime<Utc>,
}

/// Structured smoke evidence for a live Matrix connector.
#[derive(Debug, Clone, Default, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MatrixConnectorSmokeEvidence {
    pub smoke_evidence_id: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub tenant_id: String,
    pub connector_id: String,
    pub homeserver_binding_id: String,
    pub status: String,
    pub authorization_mode: String,
    pub owner: String,
    pub reason: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub remaining_risk: String,
    pub validated_at: DateTime<Utc>,
    pub retention_expires_at: DateTime<Utc>,
    pub redaction_status: String,
    #[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
    pub safe_evidence: BTreeMap<String, String>,
}

/// Builds the deterministic smoke evidence: a redacted "passed" record when
/// safe live is available, otherwise a redacted structured "skipped" record.
#[must_use]
pub fn build_matrix_connector_smoke_evidence(
    input: MatrixConnectorSmokeInput,
) -> MatrixConnectorSmokeEvidence {
    let mut now = input.now;
    if now == DateTime::<Utc>::default() {
        now = Utc::now();
    }
    let homeserver_binding_id = if input.homeserver_binding_id.is_empty() {
        format!("matrix_homeserver_{}", input.connector_id)
    } else {
        input.homeserver_binding_id
    };
    let smoke_evidence_id = format!("matrix_smoke_{}", input.connector_id);
    let retention_expires_at = now + Duration::hours(90 * 24);

    if !input.safe_live_available {
        return MatrixConnectorSmokeEvidence {
            smoke_evidence_id,
            tenant_id: input.tenant_id,
            connector_id: input.connector_id,
            homeserver_binding_id,
            status: "skipped".to_string(),
            authorization_mode: "unavailable".to_string(),
            owner: input.owner,
            reason: input.reason,
            remaining_risk: "No live Matrix hosted smoke was run; release review must consume this structured skip.".to_string(),
            validated_at: now,
            retention_expires_at,
            redaction_status: "redacted".to_string(),
            safe_evidence: BTreeMap::from([("policy".to_string(), "structured_skip".to_string())]),
        };
    }

    MatrixConnectorSmokeEvidence {
        smoke_evidence_id,
        tenant_id: input.tenant_id,
        connector_id: input.connector_id,
        homeserver_binding_id,
        status: "passed".to_string(),
        authorization_mode: "safe_live".to_string(),
        owner: input.owner,
        reason: "healthy".to_string(),
        remaining_risk: String::new(),
        validated_at: now,
        retention_expires_at,
        redaction_status: "redacted".to_string(),
        safe_evidence: BTreeMap::from([(
            "policy".to_string(),
            "safe_live_matrix_smoke".to_string(),
        )]),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use chrono::DateTime;
    use chrono::Utc;

    #[test]
    fn supports_structured_skip_and_safe_live_pass() {
        let now = DateTime::<Utc>::from_timestamp_secs(1_778_412_000).expect("now");

        let skip = build_matrix_connector_smoke_evidence(MatrixConnectorSmokeInput {
            tenant_id: "ten_matrix".to_string(),
            connector_id: "matrix-main".to_string(),
            owner: "operator".to_string(),
            reason: "safe Matrix credentials unavailable".to_string(),
            safe_live_available: false,
            now,
            ..MatrixConnectorSmokeInput::default()
        });
        assert_eq!(skip.status, "skipped");
        assert_eq!(skip.authorization_mode, "unavailable");
        assert!(!skip.remaining_risk.is_empty());

        let pass = build_matrix_connector_smoke_evidence(MatrixConnectorSmokeInput {
            tenant_id: "ten_matrix".to_string(),
            connector_id: "matrix-main".to_string(),
            owner: "operator".to_string(),
            safe_live_available: true,
            now,
            ..MatrixConnectorSmokeInput::default()
        });
        assert_eq!(pass.status, "passed");
        assert_eq!(pass.authorization_mode, "safe_live");
        assert_eq!(pass.reason, "healthy");
    }
}
