//! Correlation-key construction (port of `idempotency.go`).

/// Builds the side-effect correlation key: a stable, colon-joined identity
/// that a provider can use to de-duplicate a retry of the same action.
#[must_use]
pub fn correlation_key(validation_id: &str, ledger_entry_id: &str, action_ref: &str) -> String {
    let parts = [
        "live_validation",
        validation_id,
        ledger_entry_id,
        action_ref,
    ];
    let cleaned: Vec<&str> = parts
        .into_iter()
        .map(str::trim)
        .filter(|part| !part.is_empty())
        .collect();
    cleaned.join(":")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn correlation_key_joins_non_empty_parts() {
        assert_eq!(
            correlation_key("lv_1", "ledger_1", "action_1"),
            "live_validation:lv_1:ledger_1:action_1"
        );
        assert_eq!(correlation_key("lv_1", "", "  "), "live_validation:lv_1");
    }
}
