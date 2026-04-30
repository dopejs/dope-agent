package contracts_test

import (
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/opsreadiness"
)

func TestHostedReleaseEvidenceNoShipRules(t *testing.T) {
	index := hostedContractReleaseIndex()
	if err := opsreadiness.ValidateHostedReleaseEvidenceIndex(index, hostedContractNow); err != nil {
		t.Fatalf("passing release index invalid: %v", err)
	}

	index = hostedContractReleaseIndex()
	index.EvidenceLinks[0].Status = opsreadiness.StatusFail
	index.Decision = opsreadiness.ResultNoShip
	if err := opsreadiness.ValidateHostedReleaseEvidenceIndex(index, hostedContractNow); err == nil {
		t.Fatalf("expected failed evidence to block release index")
	}

	index = hostedContractReleaseIndex()
	index.EvidenceLinks[0].CommitOrVersion = "old-commit"
	index.Decision = opsreadiness.ResultNoShip
	if err := opsreadiness.ValidateHostedReleaseEvidenceIndex(index, hostedContractNow); err == nil {
		t.Fatalf("expected mismatched identity to block release index")
	}

	index = hostedContractReleaseIndex()
	index.RetentionExpiresAt = hostedContractNow.Add(-time.Hour)
	index.Decision = opsreadiness.ResultNoShip
	if err := opsreadiness.ValidateHostedReleaseEvidenceIndex(index, hostedContractNow); err == nil {
		t.Fatalf("expected expired evidence to block release index")
	}
	index.AuthorizedRetentionPolicy = "legal_hold_2026"
	if err := opsreadiness.ValidateHostedReleaseEvidenceIndex(index, hostedContractNow); err != nil {
		t.Fatalf("authorized longer retention should pass expiry check: %v", err)
	}
}
