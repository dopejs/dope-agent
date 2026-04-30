package evaluation

import "strings"

type ProductFixturePayloadValidation struct {
	Payload  map[string]any
	Status   RedactionStatus
	Evidence CandidateEvidence
}

func ValidateProductFixturePayload(input CandidateEvidenceInput) (ProductFixturePayloadValidation, error) {
	evidence, err := CandidateEvidenceFromPayload(input)
	if err != nil {
		return ProductFixturePayloadValidation{}, err
	}
	status := RedactionStatusClean
	if len(evidence.SensitiveFieldsExcluded) > 0 {
		status = RedactionStatusRedacted
	}
	if !evidence.MaterializationAllowed {
		status = RedactionStatusFailed
	}
	return ProductFixturePayloadValidation{
		Payload:  evidence.RedactedPayload,
		Status:   status,
		Evidence: evidence,
	}, nil
}

func RejectRepoManagedFixtureEdit(sourceKind string) error {
	if strings.TrimSpace(sourceKind) == "repo_fixture" || strings.TrimSpace(sourceKind) == string(SourceKindFixture) {
		return ErrEvaluationRepoFixtureImmutable
	}
	return nil
}
