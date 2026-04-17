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
