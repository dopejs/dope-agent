use chrono::Utc;
use kura_sandbox::{
    AccessRequest, BackendKind, Decision, DecisionResolution, ErrorClass, Execution,
    ExecutionStatus, FilesystemMode, NetworkMode, Profile, Result as SandboxResult, is_terminal,
};

#[test]
fn is_terminal_classifies() {
    assert!(is_terminal(ExecutionStatus::Completed));
    assert!(is_terminal(ExecutionStatus::Failed));
    assert!(is_terminal(ExecutionStatus::Cancelled));
    assert!(!is_terminal(ExecutionStatus::Pending));
    assert!(!is_terminal(ExecutionStatus::Running));
}

#[test]
fn error_class_none_is_empty_string() {
    assert_eq!(ErrorClass::None.as_str(), "");
    assert_eq!(ErrorClass::Timeout.as_str(), "timeout");
}

#[test]
fn profile_roundtrips_camel_case() {
    let profile = Profile {
        profile_id: "p1".to_string(),
        title: "Subprocess".to_string(),
        backend_kind: BackendKind::Subprocess,
        filesystem_policy: kura_sandbox::FilesystemPolicy {
            mode: FilesystemMode::Scoped,
            ..Default::default()
        },
        ..Profile::default()
    };
    let json = serde_json::to_string(&profile).unwrap();
    assert!(json.contains("profileId"));
    assert!(json.contains("backendKind"));
    assert!(json.contains("filesystemPolicy"));
    let back: Profile = serde_json::from_str(&json).unwrap();
    assert_eq!(back.profile_id, "p1");
    assert_eq!(back.backend_kind, BackendKind::Subprocess);
}

#[test]
fn execution_roundtrips_camel_case() {
    let now = Utc::now();
    let execution = Execution {
        execution_id: "sandbox_exec_roundtrip_1".to_string(),
        profile_id: "subprocess_default".to_string(),
        backend_kind: BackendKind::Subprocess,
        command: "echo".to_string(),
        args: vec!["hello".to_string()],
        cwd: "/tmp".to_string(),
        env_keys: vec!["PATH".to_string()],
        stdin_provided: true,
        timeout_ms: 30000,
        requested_by: "test".to_string(),
        resource_kind: "capability".to_string(),
        resource_id: "shell".to_string(),
        scope: "tool_call".to_string(),
        approval_id: "".to_string(),
        reason: "round trip".to_string(),
        metadata: std::collections::HashMap::from([(
            "managedProviderId".to_string(),
            "claude".to_string(),
        )]),
        access: AccessRequest {
            read_roots: vec!["/tmp".to_string()],
            write_roots: vec!["/tmp".to_string()],
            network_mode: Some(NetworkMode::Deny),
            allowed_hosts: Vec::new(),
            allowed_ports: Vec::new(),
            allow_loopback: false,
        },
        status: ExecutionStatus::Completed,
        decision: Decision {
            decision_id: "sandbox_decision_1".to_string(),
            resolution: DecisionResolution::Allow,
            matched_rules: vec!["profile:subprocess_default".to_string()],
            effective_profile_id: "subprocess_default".to_string(),
            effective_backend_kind: BackendKind::Subprocess,
            explanation: "allowed".to_string(),
            ..Decision::default()
        },
        result: SandboxResult {
            execution_id: "sandbox_exec_roundtrip_1".to_string(),
            status: ExecutionStatus::Completed,
            exit_code: Some(0),
            stdout: "hello".to_string(),
            backend_metadata: serde_json::Map::from_iter([(
                "backend".to_string(),
                serde_json::json!("subprocess"),
            )]),
            ..SandboxResult::default()
        },
        requested_at: now,
        updated_at: now,
        started_at: Some(now),
        completed_at: Some(now),
        consumer: None,
    };
    let json = serde_json::to_string(&execution).unwrap();
    assert!(json.contains("\"executionId\""));
    assert!(json.contains("\"backendKind\""));
    assert!(json.contains("\"envKeys\""));
    assert!(json.contains("\"stdinProvided\""));
    assert!(json.contains("\"requestedAt\""));
    assert!(json.contains("\"backendMetadata\""));
    let back: Execution = serde_json::from_str(&json).unwrap();
    assert_eq!(back.execution_id, "sandbox_exec_roundtrip_1");
    assert_eq!(back.status, ExecutionStatus::Completed);
    assert_eq!(back.decision.resolution, DecisionResolution::Allow);
    assert_eq!(back.result.stdout, "hello");
    assert_eq!(
        back.metadata.get("managedProviderId").map(|v| v.as_str()),
        Some("claude")
    );
    // Terminal status survives the round trip.
    assert!(is_terminal(back.status));
}
