use dope_sandbox::{
    is_terminal, BackendKind, ErrorClass, ExecutionStatus, FilesystemMode, Profile,
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
        filesystem_policy: dope_sandbox::FilesystemPolicy { mode: FilesystemMode::Scoped, ..Default::default() },
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
