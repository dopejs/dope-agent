package evaluation

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrEvaluationProductFixtureSourceRequired = errors.New("evaluation product fixture source required")
	ErrEvaluationProductFixtureNotEditable    = errors.New("evaluation product fixture is not editable")
	ErrEvaluationProductFixtureNotSelectable  = errors.New("evaluation product fixture is not selectable")
	ErrEvaluationRepoFixtureImmutable         = errors.New("repo-managed fixture is immutable from product editing")
)

type ProductFixtureInput struct {
	FixtureID       string
	TenantID        string
	DisplayName     string
	DomainClass     FixtureDomainClass
	SourceCandidate DiscoveredCandidate
	SourceEvidence  CandidateEvidence
	FixturePayload  map[string]any
	ChangeSummary   string
	CreatedBy       string
	IdempotencyKey  string
}

type FixtureRevisionInput struct {
	RevisionID         string
	FixturePayload     map[string]any
	ContentSummary     string
	ChangeSummary      string
	SourceEvidenceRefs []string
	RedactionStatus    RedactionStatus
	CreatedBy          string
}

type FixtureReviewDecision string

const (
	FixtureReviewApproved     FixtureReviewDecision = "approved"
	FixtureReviewRejected     FixtureReviewDecision = "rejected"
	FixtureReviewNeedsChanges FixtureReviewDecision = "needs_changes"
)

func CreateProductFixtureFromCandidate(input ProductFixtureInput, now time.Time) (ProductManagedFixture, FixtureRevision, error) {
	if err := validateProductFixtureInput(input); err != nil {
		return ProductManagedFixture{}, FixtureRevision{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	fixtureID := strings.TrimSpace(input.FixtureID)
	if fixtureID == "" {
		fixtureID = "product_fixture_" + strings.TrimPrefix(input.SourceCandidate.DiscoveredCandidateID, "candidate_")
	}
	revisionID := "revision_" + fixtureID + "_1"
	revision := FixtureRevision{
		RevisionID:         revisionID,
		FixtureID:          fixtureID,
		TenantID:           strings.TrimSpace(input.TenantID),
		RevisionNumber:     1,
		ContentSummary:     strings.TrimSpace(input.DisplayName),
		FixturePayload:     clonePayload(input.FixturePayload),
		ChangeSummary:      strings.TrimSpace(input.ChangeSummary),
		SourceEvidenceRefs: []string{input.SourceEvidence.EvidenceID},
		RedactionStatus:    redactionStatusDefault(input.SourceCandidate.RedactionStatus),
		CreatedBy:          strings.TrimSpace(input.CreatedBy),
		CreatedAt:          now.UTC(),
	}
	fixture := ProductManagedFixture{
		FixtureID:         fixtureID,
		TenantID:          strings.TrimSpace(input.TenantID),
		DisplayName:       strings.TrimSpace(input.DisplayName),
		DomainClass:       input.DomainClass,
		SourceKind:        string(ProductResourceDiscoveredCandidate),
		SourceRefs:        append([]SourceRef(nil), input.SourceCandidate.SourceRefs...),
		SourceCandidateID: input.SourceCandidate.DiscoveredCandidateID,
		CurrentRevisionID: revisionID,
		ReviewState:       ProductStatusDraft,
		SuppressionState:  SuppressionStateNone,
		RetentionState:    RetentionStateActive,
		CreatedBy:         strings.TrimSpace(input.CreatedBy),
		CreatedAt:         now.UTC(),
		UpdatedAt:         now.UTC(),
	}
	return fixture, revision, nil
}

func CreateProductFixtureRevision(fixture ProductManagedFixture, input FixtureRevisionInput, nextRevisionNumber int, now time.Time) (ProductManagedFixture, FixtureRevision, error) {
	if err := EnsureProductFixtureEditable(fixture); err != nil {
		return ProductManagedFixture{}, FixtureRevision{}, err
	}
	if nextRevisionNumber <= 0 {
		return ProductManagedFixture{}, FixtureRevision{}, ErrEvaluationProductInvalidBounds
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	revisionID := strings.TrimSpace(input.RevisionID)
	if revisionID == "" {
		revisionID = fmt.Sprintf("revision_%s_%d", fixture.FixtureID, nextRevisionNumber)
	}
	redactionStatus := redactionStatusDefault(input.RedactionStatus)
	revision := FixtureRevision{
		RevisionID:         revisionID,
		FixtureID:          fixture.FixtureID,
		TenantID:           fixture.TenantID,
		RevisionNumber:     nextRevisionNumber,
		ContentSummary:     strings.TrimSpace(input.ContentSummary),
		FixturePayload:     clonePayload(input.FixturePayload),
		ChangeSummary:      strings.TrimSpace(input.ChangeSummary),
		SourceEvidenceRefs: append([]string(nil), input.SourceEvidenceRefs...),
		RedactionStatus:    redactionStatus,
		CreatedBy:          strings.TrimSpace(input.CreatedBy),
		CreatedAt:          now.UTC(),
	}
	updated := fixture
	updated.CurrentRevisionID = revision.RevisionID
	updated.ReviewState = ProductStatusDraft
	updated.UpdatedAt = now.UTC()
	return updated, revision, nil
}

func ReviewProductFixture(fixture ProductManagedFixture, revisionID string, decision FixtureReviewDecision, now time.Time) (ProductManagedFixture, error) {
	if err := EnsureProductFixtureEditable(fixture); err != nil {
		return ProductManagedFixture{}, err
	}
	if strings.TrimSpace(revisionID) == "" || strings.TrimSpace(revisionID) != fixture.CurrentRevisionID {
		return ProductManagedFixture{}, ErrEvaluationProductFixtureSourceRequired
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	switch decision {
	case FixtureReviewApproved:
		fixture.ReviewState = ProductStatusApproved
	case FixtureReviewRejected:
		fixture.ReviewState = ProductStatusRejected
	case FixtureReviewNeedsChanges:
		fixture.ReviewState = ProductStatusDraft
	default:
		return ProductManagedFixture{}, ErrEvaluationProductInvalidBounds
	}
	fixture.UpdatedAt = now.UTC()
	return fixture, nil
}

func SuppressProductFixture(fixture ProductManagedFixture, now time.Time) (ProductManagedFixture, error) {
	if err := EnsureProductFixtureEditable(fixture); err != nil {
		return ProductManagedFixture{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	fixture.SuppressionState = SuppressionStateSuppressed
	fixture.UpdatedAt = now.UTC()
	return fixture, nil
}

func ApplyProductFixtureRetention(fixture ProductManagedFixture, state RetentionState, now time.Time) (ProductManagedFixture, error) {
	if err := EnsureProductFixtureEditable(fixture); err != nil {
		return ProductManagedFixture{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	switch state {
	case RetentionStateActive, RetentionStateExpired, RetentionStateDeleted, RetentionStateTombstone:
	default:
		return ProductManagedFixture{}, ErrEvaluationProductInvalidBounds
	}
	fixture.RetentionState = state
	if state == RetentionStateDeleted || state == RetentionStateTombstone {
		fixture.ReviewState = ProductStatusDeleted
	}
	fixture.UpdatedAt = now.UTC()
	return fixture, nil
}

func EnsureProductFixtureEditable(fixture ProductManagedFixture) error {
	if strings.TrimSpace(fixture.FixtureID) == "" || strings.TrimSpace(fixture.TenantID) == "" {
		return ErrEvaluationProductFixtureSourceRequired
	}
	if fixture.SourceKind == "repo_fixture" || fixture.SourceKind == string(SourceKindFixture) {
		return ErrEvaluationRepoFixtureImmutable
	}
	if fixture.RetentionState == RetentionStateDeleted || fixture.RetentionState == RetentionStateTombstone || fixture.ReviewState == ProductStatusDeleted {
		return ErrEvaluationProductFixtureNotEditable
	}
	return nil
}

func ProductFixtureSelectable(fixture ProductManagedFixture) error {
	if fixture.ReviewState != ProductStatusApproved ||
		fixture.SuppressionState == SuppressionStateSuppressed ||
		fixture.RetentionState != RetentionStateActive {
		return ErrEvaluationProductFixtureNotSelectable
	}
	return nil
}

func validateProductFixtureInput(input ProductFixtureInput) error {
	if err := ValidateTenantScopedProductRequest(input.TenantID); err != nil {
		return err
	}
	if strings.TrimSpace(input.DisplayName) == "" || input.DomainClass == "" {
		return ErrEvaluationProductFixtureSourceRequired
	}
	if strings.TrimSpace(input.SourceCandidate.DiscoveredCandidateID) == "" || strings.TrimSpace(input.SourceEvidence.EvidenceID) == "" {
		return ErrEvaluationProductFixtureSourceRequired
	}
	if strings.TrimSpace(input.SourceCandidate.TenantID) != strings.TrimSpace(input.TenantID) || strings.TrimSpace(input.SourceEvidence.TenantID) != strings.TrimSpace(input.TenantID) {
		return ErrEvaluationProductCrossTenantSource
	}
	if input.SourceCandidate.SuppressionState == SuppressionStateSuppressed ||
		input.SourceCandidate.RetentionState != RetentionStateActive ||
		input.SourceCandidate.RedactionStatus == RedactionStatusFailed ||
		!input.SourceEvidence.MaterializationAllowed {
		return ErrEvaluationProductFixtureNotSelectable
	}
	return nil
}

func clonePayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	out := make(map[string]any, len(payload))
	for key, value := range payload {
		out[key] = value
	}
	return out
}
