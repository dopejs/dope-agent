package contracts_test

import (
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/opsreadiness"
)

var hostedContractNow = time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)

func hostedContractRun() opsreadiness.HostedRun {
	return opsreadiness.HostedRun{
		RunID:              "hosted_contract_run",
		ProfileID:          "profile_hosted_test",
		CommitOrVersion:    "028-hosted-operational-profile",
		Host:               "stable-test-host-1",
		Operator:           "operator@example.test",
		StartedAt:          hostedContractNow.Add(-10 * time.Minute),
		SupervisorMode:     opsreadiness.HostedSupervisorModeRepoForeground,
		Status:             opsreadiness.HostedRunStatusRunning,
		ArtifactRoot:       "/tmp/hosted/artifacts/hosted_contract_run",
		RetentionExpiresAt: hostedContractNow.AddDate(0, 0, 90),
	}
}

func hostedContractLinks() []opsreadiness.HostedEvidenceLink {
	run := hostedContractRun()
	links := make([]opsreadiness.HostedEvidenceLink, 0, len(opsreadiness.RequiredHostedEvidenceTypes))
	for _, evidenceType := range opsreadiness.RequiredHostedEvidenceTypes {
		links = append(links, opsreadiness.HostedEvidenceLink{
			EvidenceType:       evidenceType,
			Path:               "/tmp/hosted/artifacts/hosted_contract_run/" + evidenceType + ".json",
			RunID:              run.RunID,
			ProfileID:          run.ProfileID,
			CommitOrVersion:    run.CommitOrVersion,
			Status:             opsreadiness.StatusPass,
			GeneratedAt:        hostedContractNow,
			RetentionExpiresAt: run.RetentionExpiresAt,
			RedactionStatus:    opsreadiness.HostedRedactionPassed,
		})
	}
	return links
}

func hostedContractReleaseIndex() opsreadiness.HostedReleaseEvidenceIndex {
	run := hostedContractRun()
	return opsreadiness.HostedReleaseEvidenceIndex{
		ReleaseIndexID:     "release_contract_1",
		RunID:              run.RunID,
		ProfileID:          run.ProfileID,
		CommitOrVersion:    run.CommitOrVersion,
		GeneratedAt:        hostedContractNow,
		ReviewTarget:       "Roadmap 43 release",
		RetentionExpiresAt: run.RetentionExpiresAt,
		Decision:           opsreadiness.ResultShip,
		ReviewElapsed:      25 * time.Minute,
		EvidenceLinks:      hostedContractLinks(),
	}
}
