use dope_feishulark::{
    adapter_failure_kind, ambiguous_fault, feishu_code_fault, http_status_fault, parse_token,
    FaultKind, ScopedToken, AMBIGUOUS_CODE,
};

#[test]
fn parse_token_fails_closed_on_empty() {
    let err = parse_token(&[]).unwrap_err();
    assert_eq!(err.kind, FaultKind::Auth);
    assert_eq!(err.code, "access_token_missing");
}

#[test]
fn parse_token_fails_closed_on_missing_token() {
    let err = parse_token(br#"{"tokenType":"user"}"#).unwrap_err();
    assert_eq!(err.code, "access_token_missing");
}

#[test]
fn parse_token_decodes_envelope() {
    let token = parse_token(br#"{"accessToken":"secret","grantedScopes":["calendar:write"]}"#).unwrap();
    assert_eq!(token.access_token, "secret");
    assert_eq!(token.granted_scopes, vec!["calendar:write"]);
}

#[test]
fn has_scope_empty_grants() {
    let token = ScopedToken::default();
    assert!(token.has_scope("calendar:write"));
}

#[test]
fn has_scope_matches_case_insensitive() {
    let token = ScopedToken {
        granted_scopes: vec!["Calendar:Write".to_string()],
        ..ScopedToken::default()
    };
    assert!(token.has_scope("calendar:write"));
    assert!(!token.has_scope("mail:read"));
}

#[test]
fn http_status_fault_mapping() {
    assert!(http_status_fault(200, false).is_none());
    assert_eq!(http_status_fault(401, false).unwrap().code, "token_expired");
    assert_eq!(http_status_fault(403, false).unwrap().kind, FaultKind::Scope);
    assert_eq!(http_status_fault(429, false).unwrap().kind, FaultKind::RateLimited);
    assert!(http_status_fault(500, true).unwrap().is_ambiguous());
    assert_eq!(http_status_fault(500, false).unwrap().code, "service_unavailable");
    assert_eq!(http_status_fault(418, false).unwrap().code, "provider_unavailable");
}

#[test]
fn feishu_code_fault_mapping() {
    assert_eq!(feishu_code_fault(99991669).code, "scope_not_granted");
    assert_eq!(feishu_code_fault(99991677).code, "token_expired");
    assert_eq!(feishu_code_fault(429).kind, FaultKind::RateLimited);
    assert_eq!(feishu_code_fault(12345).code, "provider_error_12345");
}

#[test]
fn adapter_failure_kind_mapping() {
    assert_eq!(adapter_failure_kind(FaultKind::Auth).as_str(), "auth");
    assert_eq!(adapter_failure_kind(FaultKind::Scope).as_str(), "scope");
    assert_eq!(adapter_failure_kind(FaultKind::Internal).as_str(), "internal");
}

#[test]
fn ambiguous_fault_is_marked() {
    let fault = ambiguous_fault("boom");
    assert!(fault.is_ambiguous());
    assert_eq!(fault.code, AMBIGUOUS_CODE);
}
