use std::sync::Arc;

use dope_audit::{AuditError, EVENT_CATEGORY, EVENT_NAME, Emitter};
use dope_events::{Bus, Filter};
use dope_identity::tenantctx;
use dope_identity::TenantContext;

#[test]
fn emit_requires_tenant_context() {
    let emitter = Emitter::new(Arc::new(Bus::new()));
    assert_eq!(emitter.emit("api", "run"), Err(AuditError::NoActingTenant));
}

#[test]
fn emit_publishes_denial_event() {
    let bus = Arc::new(Bus::new());
    let emitter = Emitter::new(Arc::clone(&bus));
    let tc = TenantContext {
        principal_id: "principal_1".to_string(),
        tenant_id: "tenant_1".to_string(),
        ..TenantContext::default()
    };
    tenantctx::with_context(tc, || {
        emitter.emit("api:GET /v1/runs/{id}", "run").unwrap();
    });

    let events = bus.list(&Filter {
        category: EVENT_CATEGORY.to_string(),
        ..Filter::default()
    });
    assert_eq!(events.len(), 1);
    assert_eq!(events[0].name, EVENT_NAME);
    assert_eq!(events[0].tenant_id, "tenant_1");
    assert_eq!(events[0].resource.kind, "tenant");
    assert_eq!(events[0].payload["actingTenantId"], "tenant_1");
    assert_eq!(events[0].payload["surface"], "api:GET /v1/runs/{id}");
    assert_eq!(events[0].payload["resourceKind"], "run");
}
