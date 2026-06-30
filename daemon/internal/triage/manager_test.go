package triage

import (
	"errors"
	"testing"
)

func samplePolicy(t *testing.T) (*Manager, Policy) {
	t.Helper()
	m := NewManager("test")
	policy, err := m.CreatePolicy("inbox", []Rule{
		{Description: "block spam", Conditions: []Condition{{Field: FieldSender, Operator: OperatorContains, Value: "spam@"}}, Classification: ClassificationBlocked, Outcome: OutcomeNoAction},
		{Description: "newsletters", Conditions: []Condition{{Field: FieldSender, Operator: OperatorContains, Value: "newsletter"}}, Classification: ClassificationNewsletter, Outcome: OutcomeDeliveryDigest},
		{Description: "boss urgent", Conditions: []Condition{{Field: FieldSender, Operator: OperatorContains, Value: "boss@"}, {Field: FieldSubject, Operator: OperatorContains, Value: "urgent"}}, Classification: ClassificationUrgent, Outcome: OutcomeReminder},
		{Description: "questions need reply", Conditions: []Condition{{Field: FieldSubject, Operator: OperatorContains, Value: "?"}}, Classification: ClassificationNeedsReply, Outcome: OutcomeDraftReply},
	}, ClassificationFYI)
	if err != nil {
		t.Fatalf("CreatePolicy: %v", err)
	}
	return m, policy
}

// FR-001..FR-005, SC-001/SC-002: each message gets one transparent decision from the first
// matching rule; outcomes are proposals; default applies when nothing matches.
func TestTriageRunClassifies(t *testing.T) {
	m, policy := samplePolicy(t)
	msgs := []Message{
		{MessageID: "m1", Sender: "spam@bad.example", Subject: "win money"},
		{MessageID: "m2", Sender: "weekly-newsletter@news.example", Subject: "this week"},
		{MessageID: "m3", Sender: "boss@corp.example", Subject: "URGENT: review"},
		{MessageID: "m4", Sender: "alice@corp.example", Subject: "can you help?"},
		{MessageID: "m5", Sender: "bob@corp.example", Subject: "fyi notes"},
	}
	run, err := m.Run(policy.PolicyID, msgs)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(run.Decisions) != 5 {
		t.Fatalf("want 5 decisions, got %d", len(run.Decisions))
	}
	want := []struct {
		class   Classification
		outcome Outcome
	}{
		{ClassificationBlocked, OutcomeNoAction},
		{ClassificationNewsletter, OutcomeDeliveryDigest},
		{ClassificationUrgent, OutcomeReminder},
		{ClassificationNeedsReply, OutcomeDraftReply},
		{ClassificationFYI, OutcomeNoAction},
	}
	for i, d := range run.Decisions {
		if d.Classification != want[i].class || d.Outcome != want[i].outcome {
			t.Fatalf("decision %d = %s/%s, want %s/%s", i, d.Classification, d.Outcome, want[i].class, want[i].outcome)
		}
		if !d.ReplayCandidate {
			t.Fatalf("decision %d must be a replay candidate", i)
		}
	}
	// Transparency: matched rule + evidence recorded for matched decisions; default flagged.
	if run.Decisions[0].MatchedRuleID == "" || len(run.Decisions[0].MatchedEvidence) == 0 {
		t.Fatalf("matched decision missing rule/evidence: %+v", run.Decisions[0])
	}
	if !run.Decisions[4].DefaultApplied {
		t.Fatalf("unmatched decision should flag default: %+v", run.Decisions[4])
	}
}

// SC-004: re-running the same policy on the same messages yields identical decisions.
func TestTriageRunDeterministic(t *testing.T) {
	m, policy := samplePolicy(t)
	msgs := []Message{{MessageID: "m1", Sender: "boss@corp.example", Subject: "urgent please"}}
	a, _ := m.Run(policy.PolicyID, msgs)
	b, _ := m.Run(policy.PolicyID, msgs)
	if a.Decisions[0].Classification != b.Decisions[0].Classification ||
		a.Decisions[0].MatchedRuleID != b.Decisions[0].MatchedRuleID ||
		a.Decisions[0].Outcome != b.Decisions[0].Outcome {
		t.Fatalf("triage not deterministic: %+v vs %+v", a.Decisions[0], b.Decisions[0])
	}
}

// FR-001: invalid classifications/outcomes are rejected; unknown policy run errors.
func TestTriageValidation(t *testing.T) {
	m := NewManager("test")
	if _, err := m.CreatePolicy("bad", []Rule{{Classification: Classification("bogus")}}, ClassificationFYI); !errors.Is(err, ErrInvalidRule) {
		t.Fatalf("expected invalid rule, got %v", err)
	}
	if _, err := m.Run("nope", nil); !errors.Is(err, ErrPolicyNotFound) {
		t.Fatalf("expected policy-not-found, got %v", err)
	}
}

// FR-002: first-match-wins ordering is deterministic.
func TestTriageFirstMatchWins(t *testing.T) {
	m := NewManager("test")
	policy, err := m.CreatePolicy("ordered", []Rule{
		{Conditions: []Condition{{Field: FieldSubject, Operator: OperatorContains, Value: "report"}}, Classification: ClassificationNeedsReply, Outcome: OutcomeDraftReply},
		{Conditions: []Condition{{Field: FieldSubject, Operator: OperatorContains, Value: "report"}}, Classification: ClassificationFYI, Outcome: OutcomeNoAction},
	}, ClassificationFYI)
	if err != nil {
		t.Fatalf("CreatePolicy: %v", err)
	}
	run, _ := m.Run(policy.PolicyID, []Message{{MessageID: "m", Subject: "weekly report"}})
	if run.Decisions[0].Classification != ClassificationNeedsReply {
		t.Fatalf("first match should win: %+v", run.Decisions[0])
	}
}
