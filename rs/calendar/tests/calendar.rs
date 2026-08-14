use dope_calendar::{
    attendee_emails, build_attendee_outcome, live_validation_matrix_rows, normalize_timezone,
    resolve_attendee_requests, Attendee, AttendeeRequest, InvitationStatus, NotificationBehavior,
    RecurrenceScope,
};

#[test]
fn resolve_attendee_requests_synthesizes_from_emails() {
    let requests = resolve_attendee_requests(&[], &["a@x.com".to_string(), " b@x.com ".to_string()]);
    assert_eq!(requests.len(), 2);
    assert_eq!(requests[0].email, "a@x.com");
    assert_eq!(requests[1].email, "b@x.com");
    assert_eq!(requests[0].role, "required");
}

#[test]
fn resolve_attendee_requests_prefers_explicit() {
    let req = vec![AttendeeRequest { email: "a@x.com".to_string(), ..AttendeeRequest::default() }];
    let requests = resolve_attendee_requests(&req, &["ignored@x.com".to_string()]);
    assert_eq!(requests.len(), 1);
    assert_eq!(requests[0].role, "required");
}

#[test]
fn attendee_emails_and_outcome() {
    let details = vec![Attendee {
        email: "a@x.com".to_string(),
        invitation_status: InvitationStatus::Unsupported.as_str().to_string(),
        ..Attendee::default()
    }];
    assert_eq!(attendee_emails(&details), vec!["a@x.com"]);
    let outcome = build_attendee_outcome(true, &details).expect("outcome");
    assert!(outcome.unsupported);
    assert_eq!(outcome.notification_behavior, NotificationBehavior::Unsupported);
    assert!(build_attendee_outcome(false, &[]).is_none());
}

#[test]
fn normalize_timezone_falls_back_to_utc() {
    assert_eq!(normalize_timezone("Asia/Shanghai", "UTC"), "Asia/Shanghai");
    assert_eq!(normalize_timezone("", "Asia/Tokyo"), "Asia/Tokyo");
    assert_eq!(normalize_timezone("", ""), "UTC");
}

#[test]
fn recurrence_scope_validity() {
    assert!(RecurrenceScope::ThisOccurrence.valid());
    assert!(RecurrenceScope::ThisAndFollowing.valid());
    assert!(RecurrenceScope::EntireSeries.valid());
    assert!(!RecurrenceScope::Unspecified.valid());
}

#[test]
fn live_validation_rows_cover_calendar_classes() {
    assert_eq!(live_validation_matrix_rows().len(), 4);
}
