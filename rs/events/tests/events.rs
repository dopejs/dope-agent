use dope_events::{Event, Filter, Resource, Scope, is_global_category};

#[test]
fn event_round_trips_camel_case() {
    let now = chrono::Utc::now();
    let mut payload = serde_json::Map::new();
    payload.insert("k".to_string(), serde_json::json!("v"));
    let event = Event {
        event_id: "evt_1".to_string(),
        sequence: 7,
        environment_scope: "test".to_string(),
        tenant_id: "tenant_1".to_string(),
        category: "audit".to_string(),
        name: "audit.cross_tenant_access_denied".to_string(),
        occurred_at: now,
        scope: Scope {
            run_id: "run_1".to_string(),
            ..Scope::default()
        },
        resource: Resource { kind: "run".to_string(), id: "run_1".to_string() },
        payload: payload.clone(),
    };

    let json = serde_json::to_value(&event).unwrap();
    assert_eq!(json.get("environmentScope"), None);
    assert_eq!(json["tenantId"], "tenant_1");
    assert_eq!(json["scope"]["runId"], "run_1");
    assert_eq!(json["resource"]["kind"], "run");
    assert_eq!(json["payload"]["k"], "v");

    let back: Event = serde_json::from_value(json).unwrap();
    assert_eq!(back.sequence, 7);
    assert_eq!(back.category, "audit");
    assert_eq!(back.payload.get("k"), Some(&serde_json::json!("v")));
}

#[test]
fn global_categories_are_recognized() {
    assert!(is_global_category("mcp"));
    assert!(is_global_category("daemon.migration"));
    assert!(!is_global_category("audit"));
}

#[test]
fn filter_defaults_are_empty() {
    let f = Filter::default();
    assert_eq!(f.category, "");
    assert_eq!(f.cursor, 0);
    assert!(!f.include_global);
}
