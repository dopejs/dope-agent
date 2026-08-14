use dope_mail::{
    apply_attachment_policy, evaluate_attachment, join_recipients, live_validation_matrix_rows,
    summarize_draft_input, validate_explicit_recipients, AttachmentReference,
    AttachmentResolutionStatus, MailError, MAX_ATTACHMENT_BYTES,
};

#[test]
fn join_recipients_trims_and_drops_empty() {
    let out = join_recipients(&[
        &["a@x.com".to_string(), "  b@x.com ".to_string()],
        &["".to_string(), "c@x.com".to_string()],
    ]);
    assert_eq!(out, vec!["a@x.com", "b@x.com", "c@x.com"]);
}

#[test]
fn summarize_draft_input_shows_recipients() {
    let to = vec!["a@x.com".to_string()];
    assert_eq!(summarize_draft_input("Hi", &to, &[], &[]), "Hi -> a@x.com");
    assert_eq!(summarize_draft_input("Subject only", &[], &[], &[]), "Subject only");
}

#[test]
fn validate_explicit_recipients_requires_one() {
    assert!(validate_explicit_recipients(&[]).is_err());
    assert!(matches!(
        validate_explicit_recipients(&["".to_string()]),
        Err(MailError::MailRecipientRequired)
    ));
    assert!(validate_explicit_recipients(&["a@x.com".to_string()]).is_ok());
}

#[test]
fn evaluate_attachment_resolves_standard() {
    let result = evaluate_attachment("report.pdf", "application/pdf", 100);
    assert_eq!(result.status, AttachmentResolutionStatus::Resolved);
    assert_eq!(result.retention_class, "standard");
    assert!(!result.redacted);
}

#[test]
fn evaluate_attachment_blocks_executable_extension() {
    let result = evaluate_attachment("evil.exe", "application/octet-stream", 100);
    assert_eq!(result.status, AttachmentResolutionStatus::Failed);
    assert!(result.failure_reason.contains("unsupported_type"));
}

#[test]
fn evaluate_attachment_blocks_too_large() {
    let result = evaluate_attachment("big.bin", "application/octet-stream", MAX_ATTACHMENT_BYTES + 1);
    assert_eq!(result.status, AttachmentResolutionStatus::Failed);
    assert!(result.failure_reason.contains("too_large"));
}

#[test]
fn apply_attachment_policy_stamps_reference() {
    let mut reference = AttachmentReference {
        display_name: "script.sh".to_string(),
        media_type: "application/x-sh".to_string(),
        size_bytes: Some(10),
        ..AttachmentReference::default()
    };
    apply_attachment_policy(&mut reference);
    assert_eq!(reference.resolution_status, AttachmentResolutionStatus::Failed);
    assert!(reference.failure_reason.contains("unsupported_type"));
}

#[test]
fn live_validation_rows_cover_mail_classes() {
    assert_eq!(live_validation_matrix_rows().len(), 5);
}
