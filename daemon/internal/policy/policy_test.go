package policy

import (
	"errors"
	"testing"
)

func TestRequestAndResolveApproval(t *testing.T) {
	engine := NewEngine()

	approval, decision, err := engine.RequestApproval(RequestApprovalInput{
		Action:       "capability.exec",
		ResourceKind: "capability",
		ResourceID:   "shell",
		Reason:       "shell execution requires operator approval",
		RequestedBy:  "operator",
	})
	if err != nil {
		t.Fatalf("RequestApproval returned error: %v", err)
	}
	if approval.Status != ApprovalStatusPending {
		t.Fatalf("expected pending approval status, got %s", approval.Status)
	}
	if decision.Outcome != DecisionOutcomeRequiresApproval {
		t.Fatalf("expected requires_approval decision outcome, got %s", decision.Outcome)
	}

	items := engine.ListApprovals(ApprovalStatusPending)
	if len(items) != 1 {
		t.Fatalf("expected 1 pending approval, got %d", len(items))
	}

	resolved, finalDecision, err := engine.ResolveApproval(approval.ApprovalID, ResolveApprovalInput{
		Resolution: string(ApprovalStatusApproved),
		Comment:    "approved for local execution",
	})
	if err != nil {
		t.Fatalf("ResolveApproval returned error: %v", err)
	}
	if resolved.Status != ApprovalStatusApproved {
		t.Fatalf("expected approved status, got %s", resolved.Status)
	}
	if finalDecision.Outcome != DecisionOutcomeApproved {
		t.Fatalf("expected approved decision outcome, got %s", finalDecision.Outcome)
	}
}

func TestResolveApprovalRejectsInvalidState(t *testing.T) {
	engine := NewEngine()

	_, _, err := engine.ResolveApproval("approval_missing", ResolveApprovalInput{
		Resolution: string(ApprovalStatusApproved),
	})
	if !errors.Is(err, ErrApprovalNotFound) {
		t.Fatalf("expected ErrApprovalNotFound, got %v", err)
	}

	approval, _, err := engine.RequestApproval(RequestApprovalInput{
		Action: "capability.exec",
		Reason: "needs approval",
	})
	if err != nil {
		t.Fatalf("RequestApproval returned error: %v", err)
	}

	_, _, err = engine.ResolveApproval(approval.ApprovalID, ResolveApprovalInput{
		Resolution: "maybe",
	})
	if !errors.Is(err, ErrInvalidResolution) {
		t.Fatalf("expected ErrInvalidResolution, got %v", err)
	}
}

func TestApprovalsStayScopedToRequestedResourceInstance(t *testing.T) {
	engine := NewEngine()

	shellApproval, _, err := engine.RequestApproval(RequestApprovalInput{
		Action:       "tool_call.execute",
		ResourceKind: "capability",
		ResourceID:   "shell",
		Reason:       "shell requires approval",
		RequestedBy:  "operator",
	})
	if err != nil {
		t.Fatalf("RequestApproval(shell) returned error: %v", err)
	}
	browserApproval, _, err := engine.RequestApproval(RequestApprovalInput{
		Action:       "tool_call.execute",
		ResourceKind: "capability",
		ResourceID:   "browser",
		Reason:       "browser requires approval",
		RequestedBy:  "operator",
	})
	if err != nil {
		t.Fatalf("RequestApproval(browser) returned error: %v", err)
	}

	resolved, _, err := engine.ResolveApproval(shellApproval.ApprovalID, ResolveApprovalInput{
		Resolution: string(ApprovalStatusApproved),
		Comment:    "approve shell only",
	})
	if err != nil {
		t.Fatalf("ResolveApproval(shell) returned error: %v", err)
	}
	if resolved.ResourceID != "shell" || resolved.Status != ApprovalStatusApproved {
		t.Fatalf("expected resolved shell approval, got %+v", resolved)
	}

	browser, ok := engine.GetApproval(browserApproval.ApprovalID)
	if !ok {
		t.Fatalf("expected browser approval %s", browserApproval.ApprovalID)
	}
	if browser.ResourceID != "browser" || browser.Status != ApprovalStatusPending {
		t.Fatalf("expected browser approval to remain pending and isolated, got %+v", browser)
	}

	pending := engine.ListApprovals(ApprovalStatusPending)
	if len(pending) != 1 || pending[0].ApprovalID != browserApproval.ApprovalID {
		t.Fatalf("expected only browser approval to remain pending, got %+v", pending)
	}
}
