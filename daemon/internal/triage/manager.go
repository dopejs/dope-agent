package triage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

var (
	ErrPolicyNotFound = errors.New("triage policy not found")
	ErrInvalidRule    = errors.New("triage rule is invalid")
	ErrInvalidPolicy  = errors.New("triage policy is invalid")
)

// Manager owns triage policies and evaluates triage runs. Policies are in-memory with Restore,
// mirroring other rule/resource managers; runs are pure functions of (policy, messages) so they
// are deterministic and replayable (FR-006).
type Manager struct {
	mu       sync.RWMutex
	env      string
	policies map[string]Policy
}

func NewManager(environmentScope string) *Manager {
	return &Manager{env: strings.TrimSpace(environmentScope), policies: make(map[string]Policy)}
}

// Restore reloads persisted policies.
func (m *Manager) Restore(policies []Policy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policies = make(map[string]Policy, len(policies))
	for _, p := range policies {
		m.policies[p.PolicyID] = p
	}
}

// CreatePolicy validates and stores a policy. Classifications and outcomes must be from the fixed
// sets; the default classification defaults to fyi.
func (m *Manager) CreatePolicy(name string, rules []Rule, defaultClassification Classification) (Policy, error) {
	normalized, err := normalizeRules(rules)
	if err != nil {
		return Policy{}, err
	}
	if defaultClassification == "" {
		defaultClassification = ClassificationFYI
	}
	if !defaultClassification.valid() {
		return Policy{}, ErrInvalidPolicy
	}
	now := time.Now().UTC()
	policy := Policy{
		PolicyID:              newID("triage_policy"),
		EnvironmentScope:      m.env,
		Name:                  strings.TrimSpace(name),
		Rules:                 normalized,
		DefaultClassification: defaultClassification,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	m.mu.Lock()
	m.policies[policy.PolicyID] = policy
	m.mu.Unlock()
	return policy, nil
}

func (m *Manager) GetPolicy(policyID string) (Policy, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.policies[strings.TrimSpace(policyID)]
	return p, ok
}

func (m *Manager) ListPolicies() []Policy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Policy, 0, len(m.policies))
	for _, p := range m.policies {
		out = append(out, p)
	}
	return out
}

// Run evaluates messages against a policy and returns a triage run with one decision per message.
func (m *Manager) Run(policyID string, messages []Message) (Run, error) {
	policy, ok := m.GetPolicy(policyID)
	if !ok {
		return Run{}, ErrPolicyNotFound
	}
	now := time.Now().UTC()
	decisions := make([]Decision, 0, len(messages))
	for _, msg := range messages {
		decisions = append(decisions, evaluate(policy, msg, now))
	}
	return Run{
		RunID:            newID("triage_run"),
		PolicyID:         policy.PolicyID,
		EnvironmentScope: policy.EnvironmentScope,
		MessageCount:     len(messages),
		Decisions:        decisions,
		CreatedAt:        now,
	}, nil
}

// evaluate applies the first fully-matching rule (deterministic order); when none match the
// default classification is applied with a no-action outcome. The decision records the matched
// rule + evidence so the classification is transparent (FR-004), and is always a replay
// candidate (FR-006).
func evaluate(policy Policy, msg Message, now time.Time) Decision {
	for _, rule := range policy.Rules {
		if evidence, matched := matchRule(rule, msg); matched {
			return Decision{
				MessageID:       msg.MessageID,
				Classification:  rule.Classification,
				MatchedRuleID:   rule.RuleID,
				MatchedEvidence: evidence,
				Outcome:         rule.Outcome,
				ReplayCandidate: true,
				DecidedAt:       now,
			}
		}
	}
	return Decision{
		MessageID:       msg.MessageID,
		Classification:  policy.DefaultClassification,
		Outcome:         OutcomeNoAction,
		DefaultApplied:  true,
		ReplayCandidate: true,
		DecidedAt:       now,
	}
}

// matchRule reports whether every condition matches (AND); an empty condition set is a catch-all.
func matchRule(rule Rule, msg Message) ([]MatchedEvidence, bool) {
	evidence := make([]MatchedEvidence, 0, len(rule.Conditions))
	for _, cond := range rule.Conditions {
		if !matchCondition(cond, msg) {
			return nil, false
		}
		evidence = append(evidence, MatchedEvidence{Field: cond.Field, Operator: cond.Operator, Value: cond.Value})
	}
	return evidence, true
}

func matchCondition(cond Condition, msg Message) bool {
	want := strings.ToLower(strings.TrimSpace(cond.Value))
	switch cond.Field {
	case FieldSender:
		return compareString(cond.Operator, msg.Sender, want)
	case FieldSubject:
		return compareString(cond.Operator, msg.Subject, want)
	case FieldBody:
		return compareString(cond.Operator, msg.BodyPreview, want)
	case FieldRecipient:
		return compareList(cond.Operator, msg.Recipients, want)
	default:
		return false
	}
}

func compareString(op ConditionOperator, field, want string) bool {
	have := strings.ToLower(strings.TrimSpace(field))
	switch op {
	case OperatorContains:
		return want != "" && strings.Contains(have, want)
	case OperatorEquals:
		return have == want
	case OperatorNotContains:
		return want == "" || !strings.Contains(have, want)
	default:
		return false
	}
}

func compareList(op ConditionOperator, fields []string, want string) bool {
	switch op {
	case OperatorContains:
		for _, f := range fields {
			if want != "" && strings.Contains(strings.ToLower(strings.TrimSpace(f)), want) {
				return true
			}
		}
		return false
	case OperatorEquals:
		for _, f := range fields {
			if strings.ToLower(strings.TrimSpace(f)) == want {
				return true
			}
		}
		return false
	case OperatorNotContains:
		for _, f := range fields {
			if want != "" && strings.Contains(strings.ToLower(strings.TrimSpace(f)), want) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func normalizeRules(rules []Rule) ([]Rule, error) {
	out := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		if !rule.Classification.valid() {
			return nil, ErrInvalidRule
		}
		if rule.Outcome == "" {
			rule.Outcome = OutcomeNoAction
		}
		if !rule.Outcome.valid() {
			return nil, ErrInvalidRule
		}
		for _, cond := range rule.Conditions {
			if !validField(cond.Field) || !validOperator(cond.Operator) {
				return nil, ErrInvalidRule
			}
		}
		if strings.TrimSpace(rule.RuleID) == "" {
			rule.RuleID = newID("triage_rule")
		}
		out = append(out, rule)
	}
	return out, nil
}

func validField(f ConditionField) bool {
	switch f {
	case FieldSender, FieldSubject, FieldBody, FieldRecipient:
		return true
	default:
		return false
	}
}

func validOperator(o ConditionOperator) bool {
	switch o {
	case OperatorContains, OperatorEquals, OperatorNotContains:
		return true
	default:
		return false
	}
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_fallback"
	}
	return prefix + "_" + hex.EncodeToString(buf)
}
