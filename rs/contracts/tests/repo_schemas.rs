//! Loader-level contract tests against the repository's real `schemas/`
//! tree. Ported from the self-contained tests in
//! daemon/internal/contracts/contracts_test.go:
//! - TestRequestSchemasAcceptCanonicalFixtures
//! - TestValidatorRejectsInvalidRequestFixture
//! - TestIntegrationAdapterSchemasAcceptCanonicalFixtures
//!
//! The per-domain contract tests (the remaining ~3.8k LOC of Go test files,
//! which import the whole daemon) land in wave 8 against the ported domain
//! crates.

use std::path::PathBuf;

use dope_contracts::Validator;

/// Mirrors Go's schemaRootDir: daemon/internal/contracts/../../.. is the
/// repository root; from rs/contracts that is ../...
fn schema_root_dir() -> PathBuf {
    let root = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("..")
        .join("..");
    assert!(
        root.join("schemas").is_dir(),
        "schemas/ directory not found under {}",
        root.display()
    );
    root
}

fn must_validate_fixtures(validator: &Validator, fixtures: &[(&str, &str)]) {
    for (schema_path, fixture) in fixtures {
        validator
            .validate_relative(schema_path, fixture.as_bytes())
            .unwrap_or_else(|err| panic!("ValidateRelative({schema_path}): {err}"));
    }
}

#[test]
fn request_schemas_accept_canonical_fixtures() {
    let validator = Validator::new(schema_root_dir());
    let fixtures: &[(&str, &str)] = &[
        ("schemas/api/create-run.request.schema.json", r#"{"entrypoint":"chat","goal":"ship daemon"}"#),
        ("schemas/api/connector-ingress-message.request.schema.json", r#"{"route":{"kind":"group","accountId":"bot-main","peerId":"channel-1","threadId":"thread-1"},"message":{"messageId":"msg_1","text":"hello"},"run":{"entrypoint":"connector.message","goal":"handle inbound"}}"#),
        ("schemas/api/create-step.request.schema.json", r#"{"title":"plan","kind":"task","input":{"phase":"draft"}}"#),
        ("schemas/api/update-step-status.request.schema.json", r#"{"status":"planning","output":{"phase":"ok"}}"#),
        ("schemas/api/create-tool-call.request.schema.json", r#"{"capabilityId":"docs","toolName":"lookup","approvalId":"approval_1","input":{"query":"hello"}}"#),
        ("schemas/api/complete-tool-call.request.schema.json", r#"{"output":{"ok":true}}"#),
        ("schemas/api/fail-tool-call.request.schema.json", r#"{"error":"tool failed"}"#),
        ("schemas/api/create-connector.request.schema.json", r#"{"connectorId":"telegram-main","kind":"telegram","displayName":"Telegram Main"}"#),
        ("schemas/api/report-connector-health.request.schema.json", r#"{"status":"healthy"}"#),
        ("schemas/api/report-connector-failure.request.schema.json", r#"{"reason":"socket dropped"}"#),
        ("schemas/api/create-capability.request.schema.json", r#"{"capabilityId":"docs","kind":"docs","displayName":"Docs"}"#),
        ("schemas/api/report-capability-health.request.schema.json", r#"{"status":"degraded"}"#),
        ("schemas/api/report-capability-failure.request.schema.json", r#"{"reason":"worker exited"}"#),
        ("schemas/api/create-llm-dispatch.request.schema.json", r#"{"provider":"echo","model":"echo-v1","messages":[{"role":"user","content":"hello"}],"timeoutMs":1000,"maxRetries":1}"#),
        ("schemas/api/chat-query.request.schema.json", r#"{"provider":"echo","model":"echo-v1","skills":["shared"],"query":"hello","timeoutMs":1000,"maxRetries":1}"#),
        ("schemas/api/run-provider-check.request.schema.json", r#"{"model":"echo-v1","prompt":"hello"}"#),
        ("schemas/api/provider-default-model.request.schema.json", r#"{"model":"gpt-5.4"}"#),
        ("schemas/api/create-schedule.request.schema.json", r#"{"trigger":{"kind":"once","fireAt":"2026-04-22T10:01:00Z"},"target":{"kind":"workflow","workflow":{"entrypoint":"operator","runGoal":"dispatch calendar workflow","workflowGoal":"create calendar event","calendarAction":{"operationClass":"create_event","integrationId":"calendar-a","title":"Calendar workflow","startsAt":"2026-04-23T17:00:00Z","endsAt":"2026-04-23T17:30:00Z"}}},"retryPolicy":{"maxRetries":1,"backoffKind":"fixed","baseDelaySeconds":5,"maxDelaySeconds":5}}"#),
        ("schemas/api/request-approval.request.schema.json", r#"{"action":"tool_call.execute","resourceKind":"capability","resourceId":"browser","reason":"needs approval","requestedBy":"web-ui"}"#),
        ("schemas/api/resolve-approval.request.schema.json", r#"{"resolution":"approved","comment":"allowed"}"#),
        ("schemas/api/create-integration.request.schema.json", r#"{"integrationId":"calendar-a","domainKind":"calendar","displayName":"Calendar A","backendKind":"fake_local","accountBinding":{"accountKey":"acct_calendar"},"canonicalDefault":true}"#),
        ("schemas/api/report-integration-readiness.request.schema.json", r#"{"readinessStatus":"healthy","authState":"authorized","healthState":"healthy","reason":"probe passed","requiredOperatorAction":"none","secretResolution":"resolved","accountBinding":{"accountKey":"acct_calendar"}}"#),
        ("schemas/api/set-integration-default.request.schema.json", r#"{}"#),
        ("schemas/api/create-integration-probe.request.schema.json", r#"{"probeKind":"mutate","approvalId":"approval_1","input":{"mode":"write"}}"#),
        ("schemas/api/create-calendar-availability-query.request.schema.json", r#"{"integrationId":"calendar-a","windowStart":"2026-04-23T16:00:00Z","windowEnd":"2026-04-23T18:00:00Z","source":{"runId":"run_1","stepId":"step_1","toolCallId":"tool_call_1","workflowId":"wf_1","workflowStepId":"wfstep_1","scheduleId":"sched_1","scheduleAttemptId":"sched_attempt_1","deliveryId":"delivery_1"}}"#),
        ("schemas/api/create-calendar-event.request.schema.json", r#"{"integrationId":"calendar-a","title":"Design review","startsAt":"2026-04-23T17:00:00Z","endsAt":"2026-04-23T17:30:00Z","timezone":"America/Los_Angeles"}"#),
        ("schemas/api/update-calendar-event.request.schema.json", r#"{"title":"Moved review","startsAt":"2026-04-23T18:00:00Z","endsAt":"2026-04-23T18:30:00Z"}"#),
        ("schemas/api/cancel-calendar-event.request.schema.json", r#"{"reason":"cancelled","source":{"runId":"run_1"}}"#),
        ("schemas/api/create-mail-draft.request.schema.json", r#"{"integrationId":"mail-a","composeMode":"new_message","to":["carol@example.com"],"subject":"Phase 30 draft","body":"Hello","source":{"runId":"run_1","workflowId":"wf_1","allowSendSideEffects":false}}"#),
        ("schemas/api/update-mail-draft.request.schema.json", r#"{"integrationId":"mail-a","subject":"Updated draft","attachmentRefs":[{"attachmentRefId":"attachment_1","displayName":"brief.pdf","mediaType":"application/pdf","sizeBytes":1024}],"source":{"runId":"run_1"}}"#),
        ("schemas/api/send-mail-message.request.schema.json", r#"{"integrationId":"mail-a","to":["carol@example.com"],"subject":"Phase 30 send","body":"Hello","source":{"workflowId":"wf_1","allowSendSideEffects":true}}"#),
        ("schemas/api/send-mail-draft.request.schema.json", r#"{"integrationId":"mail-a","source":{"scheduleId":"sched_1","scheduleAttemptId":"sched_attempt_1","allowSendSideEffects":true}}"#),
        ("schemas/api/reply-mail-message.request.schema.json", r#"{"integrationId":"mail-a","resultMode":"draft","body":"Reply later","source":{"runId":"run_1"}}"#),
        ("schemas/api/forward-mail-message.request.schema.json", r#"{"integrationId":"mail-a","resultMode":"send","to":["dave@example.com"],"body":"FYI","source":{"workflowId":"wf_1","allowSendSideEffects":true}}"#),
        ("schemas/api/sandbox-execution.request.schema.json", r#"{"profileId":"subprocess_default","command":"echo","args":["hello"],"cwd":"/tmp/dope","timeoutMs":1000,"requestedBy":"web-ui","resourceKind":"skill","resourceId":"shared","scope":"chat","reason":"inspect profile","metadata":{"ticket":"sandbox-16"},"access":{"readRoots":["/tmp/dope"],"writeRoots":["/tmp/dope"],"networkMode":"allow_list","allowedHosts":["localhost"],"allowedPorts":[80],"allowLoopback":true}}"#),
        ("schemas/api/sandbox-explain.request.schema.json", r#"{"profileId":"subprocess_default","command":"echo","args":["hello"],"cwd":"/tmp/dope","access":{"readRoots":["/tmp/dope"],"writeRoots":["/tmp/dope"],"allowedHosts":[],"allowedPorts":[]}}"#),
        ("schemas/api/mcp-server-create.request.schema.json", r#"{"serverId":"mcp-test","displayName":"MCP Test","enabled":true,"sandboxProfileId":"subprocess_default","declarationId":"mcp_server:mcp-test:lifecycle.start","transportKind":"stdio","command":"/tmp/mcp-helper","args":["--stdio"],"workingDir":"/tmp/dope","secretRefs":["MCP_TEST_TOKEN"],"autoRestart":true}"#),
        ("schemas/api/mcp-server-update.request.schema.json", r#"{"displayName":"Updated MCP","enabled":false,"autoRestart":false}"#),
        ("schemas/api/mcp-catalog-install-request.schema.json", r#"{"serverId":"filesystem-test","displayName":"Filesystem Test","workingDir":"/tmp/dope"}"#),
        ("schemas/api/mcp-tool-exposure-update.request.schema.json", r#"{"runtimeSurface":"chat","exposureMode":"approval_required","active":true,"reason":"needs approval"}"#),
        ("schemas/api/mcp-tool-authorization.request.schema.json", r#"{"runtimeSurface":"chat","approvalId":"approval_1","requestedBy":"web-ui"}"#),
        ("schemas/api/start-pairing.request.schema.json", r#"{"mode":"local","label":"web-ui","ttlSeconds":120}"#),
        ("schemas/api/complete-pairing.request.schema.json", r#"{"code":"123456"}"#),
    ];
    must_validate_fixtures(&validator, fixtures);

    validator
        .validate_relative(
            "schemas/api/create-run.request.schema.json",
            br#"{"entrypoint":"chat","route":{"kind":"direct","channel":"telegram","accountId":"bot-main","peerId":"dm-1"}}"#,
        )
        .unwrap_or_else(|err| panic!("ValidateRelative(create-run route fixture): {err}"));
}

#[test]
fn validator_rejects_invalid_request_fixture() {
    let validator = Validator::new(schema_root_dir());
    validator
        .validate_relative(
            "schemas/api/create-run.request.schema.json",
            br#"{"goal":"missing entrypoint"}"#,
        )
        .expect_err("expected invalid create-run fixture to fail schema validation");
}

#[test]
fn integration_adapter_schemas_accept_canonical_fixtures() {
    let validator = Validator::new(schema_root_dir());
    must_validate_fixtures(
        &validator,
        &[
            ("schemas/capability/integration-adapter/request.json", r#"{"requestId":"req-1","contractVersion":"1","domain":"calendar","operation":"ProjectAccount","deadlineMs":30000,"resource":{}}"#),
            ("schemas/capability/integration-adapter/response.json", r#"{"requestId":"req-1","contractVersion":"1","status":"ok","payload":{}}"#),
            ("schemas/events/integrations/adapter-health.json", r#"{"capabilityId":"cap-1","domain":"calendar","status":"healthy","readiness":"ready"}"#),
        ],
    );
}
