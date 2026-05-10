package matrix

import (
	"errors"
	"strings"
	"time"
)

type ProviderDecision struct {
	SelectedProvider        string
	RejectedProvider        string
	DecisionOwner           string
	DecisionDate            time.Time
	HostedViabilityEvidence string
	ProviderRiskEvidence    string
	UnsupportedBoundaries   []string
	ConformanceImplications string
	UnsafeMatrixDependency  bool
}

var (
	ErrDecisionOwnerRequired  = errors.New("provider decision owner is required")
	ErrWhatsAppOutOfScope     = errors.New("whatsapp is out of scope for phase 52")
	ErrUnsafeMatrixDependency = errors.New("matrix implementation depends on unsupported hosted behavior")
)

func Phase52ProviderDecision(owner string, when time.Time) ProviderDecision {
	if when.IsZero() {
		when = time.Now().UTC()
	}
	return ProviderDecision{
		SelectedProvider:        ConnectorKind,
		RejectedProvider:        "whatsapp",
		DecisionOwner:           strings.TrimSpace(owner),
		DecisionDate:            when,
		HostedViabilityEvidence: "Matrix client-server API supports tenant-provided bot accounts on tenant-selected homeservers.",
		ProviderRiskEvidence:    "WhatsApp remains rejected for hosted-safe operation in phase 52.",
		UnsupportedBoundaries:   []string{"whatsapp", "dopeagent_hosted_homeserver", "matrix_account_provisioning", "encrypted_rooms", "e2ee_key_session_management"},
		ConformanceImplications: "Matrix consumes the shared channel connector conformance contract.",
	}
}

func ValidateProviderDecision(decision ProviderDecision) error {
	if strings.TrimSpace(decision.DecisionOwner) == "" {
		return ErrDecisionOwnerRequired
	}
	if strings.TrimSpace(decision.SelectedProvider) != ConnectorKind || strings.TrimSpace(decision.RejectedProvider) != "whatsapp" {
		return ErrWhatsAppOutOfScope
	}
	if decision.UnsafeMatrixDependency {
		return ErrUnsafeMatrixDependency
	}
	return nil
}
