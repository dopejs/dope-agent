package matrix

import (
	"errors"
	"testing"
	"time"
)

func TestProviderDecisionSelectsMatrixAndRejectsWhatsApp(t *testing.T) {
	t.Parallel()

	decision := Phase52ProviderDecision("owner@example.com", time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC))
	if err := ValidateProviderDecision(decision); err != nil {
		t.Fatalf("ValidateProviderDecision returned error: %v", err)
	}
	if decision.SelectedProvider != ConnectorKind || decision.RejectedProvider != "whatsapp" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestProviderDecisionBlocksUnsafeMatrixOrWhatsAppFallback(t *testing.T) {
	t.Parallel()

	decision := Phase52ProviderDecision("owner@example.com", time.Now())
	decision.UnsafeMatrixDependency = true
	if err := ValidateProviderDecision(decision); !errors.Is(err, ErrUnsafeMatrixDependency) {
		t.Fatalf("unsafe Matrix dependency error = %v, want %v", err, ErrUnsafeMatrixDependency)
	}

	decision = Phase52ProviderDecision("owner@example.com", time.Now())
	decision.SelectedProvider = "whatsapp"
	if err := ValidateProviderDecision(decision); !errors.Is(err, ErrWhatsAppOutOfScope) {
		t.Fatalf("WhatsApp error = %v, want %v", err, ErrWhatsAppOutOfScope)
	}
}
