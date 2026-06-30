// Package triage implements the inbox triage MVP (Roadmap 65): an explicit-rule, memory-free
// workflow that classifies mail and proposes visible actions. It uses no learned preferences,
// no semantic user model, and never silently auto-sends — every decision is transparent
// (matched rule + evidence) and a deterministic replay candidate.
package triage

import "time"

// Classification is the fixed set of triage classifications (FR-003).
type Classification string

const (
	ClassificationUrgent      Classification = "urgent"
	ClassificationNeedsReply  Classification = "needs_reply"
	ClassificationFYI         Classification = "fyi"
	ClassificationNewsletter  Classification = "newsletter"
	ClassificationBlocked     Classification = "blocked"
	ClassificationUnsupported Classification = "unsupported"
)

func (c Classification) valid() bool {
	switch c {
	case ClassificationUrgent, ClassificationNeedsReply, ClassificationFYI,
		ClassificationNewsletter, ClassificationBlocked, ClassificationUnsupported:
		return true
	default:
		return false
	}
}

// Outcome is the proposed action for a classified message (FR-005). All outcomes are proposals;
// none performs a side effect without explicit downstream permission.
type Outcome string

const (
	OutcomeDraftReply     Outcome = "draft_reply"
	OutcomeReminder       Outcome = "reminder"
	OutcomeDeliveryDigest Outcome = "delivery_digest"
	OutcomeNoAction       Outcome = "no_action"
)

func (o Outcome) valid() bool {
	switch o {
	case OutcomeDraftReply, OutcomeReminder, OutcomeDeliveryDigest, OutcomeNoAction:
		return true
	default:
		return false
	}
}

// ConditionField names a message field a rule condition matches against.
type ConditionField string

const (
	FieldSender    ConditionField = "sender"
	FieldSubject   ConditionField = "subject"
	FieldBody      ConditionField = "body"
	FieldRecipient ConditionField = "recipient"
)

// ConditionOperator is how a condition compares the field to its value.
type ConditionOperator string

const (
	OperatorContains    ConditionOperator = "contains"
	OperatorEquals      ConditionOperator = "equals"
	OperatorNotContains ConditionOperator = "not_contains"
)

// Condition is a single match predicate. All conditions in a rule AND together.
type Condition struct {
	Field    ConditionField    `json:"field"`
	Operator ConditionOperator `json:"operator"`
	Value    string            `json:"value"`
}

// Rule maps a set of conditions (AND) to a classification + proposed outcome. Rules are
// operator-defined and evaluated in order; the first fully-matching rule wins.
type Rule struct {
	RuleID         string         `json:"ruleId"`
	Description    string         `json:"description,omitempty"`
	Conditions     []Condition    `json:"conditions"`
	Classification Classification `json:"classification"`
	Outcome        Outcome        `json:"outcome"`
}

// Policy is an ordered set of triage rules plus a default classification for unmatched messages.
type Policy struct {
	PolicyID              string         `json:"policyId"`
	EnvironmentScope      string         `json:"environmentScope"`
	Name                  string         `json:"name"`
	Rules                 []Rule         `json:"rules"`
	DefaultClassification Classification `json:"defaultClassification"`
	CreatedAt             time.Time      `json:"createdAt"`
	UpdatedAt             time.Time      `json:"updatedAt"`
}

// Message is the triage input projected from a mail message snapshot (no body content beyond a
// preview; no credential material).
type Message struct {
	MessageID   string   `json:"messageId"`
	ThreadID    string   `json:"threadId,omitempty"`
	Sender      string   `json:"sender,omitempty"`
	Recipients  []string `json:"recipients,omitempty"`
	Subject     string   `json:"subject,omitempty"`
	BodyPreview string   `json:"bodyPreview,omitempty"`
}

// MatchedEvidence records which condition of a rule matched, making the decision transparent.
type MatchedEvidence struct {
	Field    ConditionField    `json:"field"`
	Operator ConditionOperator `json:"operator"`
	Value    string            `json:"value"`
}

// Decision is the per-message triage outcome: classification, the matched rule (empty for the
// default), the matched evidence, and the proposed outcome. Every decision is a replay candidate.
type Decision struct {
	MessageID       string            `json:"messageId"`
	Classification  Classification    `json:"classification"`
	MatchedRuleID   string            `json:"matchedRuleId,omitempty"`
	MatchedEvidence []MatchedEvidence `json:"matchedEvidence,omitempty"`
	Outcome         Outcome           `json:"outcome"`
	DefaultApplied  bool              `json:"defaultApplied,omitempty"`
	ReplayCandidate bool              `json:"replayCandidate"`
	DecidedAt       time.Time         `json:"decidedAt"`
}

// Run is one triage evaluation of a message set against a policy.
type Run struct {
	RunID            string     `json:"runId"`
	PolicyID         string     `json:"policyId"`
	EnvironmentScope string     `json:"environmentScope"`
	MessageCount     int        `json:"messageCount"`
	Decisions        []Decision `json:"decisions"`
	CreatedAt        time.Time  `json:"createdAt"`
}
