package contracts_test

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/opsreadiness"
)

func TestHostedRecoveryEvidenceBlocksReleaseOnFailedRestoreOrRollback(t *testing.T) {
	run := hostedContractRun()
	restore := opsreadiness.HostedRestoreRehearsalResult{
		RestoreResultID:             "restore_contract_1",
		RunID:                       run.RunID,
		BackupID:                    "backup_contract_1",
		TargetProfileID:             "profile_restore",
		TargetDataDirectory:         "/tmp/dope-restore",
		TargetIsAlternate:           true,
		TenantCount:                 3,
		TenantStates:                []opsreadiness.TenantStateSummary{{TenantID: "a", CredentialRefs: []string{"secretref_a"}, QuotaState: "ok", WorkState: "done"}, {TenantID: "b", CredentialRefs: []string{"secretref_b"}, QuotaState: "near_limit", WorkState: "pending"}, {TenantID: "c", CredentialRefs: []string{"secretref_c"}, QuotaState: "exhausted", WorkState: "operator_action"}},
		TenantStateResult:           opsreadiness.StatusPass,
		MigrationStateResult:        opsreadiness.StatusPass,
		CredentialRemediationResult: opsreadiness.StatusPass,
		QuotaStateResult:            opsreadiness.StatusPass,
		DaemonHealthResult:          opsreadiness.StatusPass,
		RawCredentialScanResult:     opsreadiness.StatusPass,
		Result:                      opsreadiness.StatusPass,
		GeneratedAt:                 hostedContractNow,
	}
	if err := opsreadiness.ValidateHostedRestoreRehearsal(run, restore); err != nil {
		t.Fatalf("passing restore rehearsal invalid: %v", err)
	}
	restore.TenantCount = 2
	restore.TenantStates = restore.TenantStates[:2]
	if err := opsreadiness.ValidateHostedRestoreRehearsal(run, restore); err == nil {
		t.Fatalf("expected fewer than three tenants to fail restore evidence")
	}

	index := hostedContractReleaseIndex()
	for i := range index.EvidenceLinks {
		if index.EvidenceLinks[i].EvidenceType == "restore_evidence" {
			index.EvidenceLinks[i].Status = opsreadiness.StatusFail
		}
	}
	index.Decision = opsreadiness.ResultNoShip
	if err := opsreadiness.ValidateHostedReleaseEvidenceIndex(index, hostedContractNow); err == nil {
		t.Fatalf("expected failed restore evidence to block release index")
	}
}
