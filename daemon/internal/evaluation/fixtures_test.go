package evaluation

import (
	"errors"
	"testing"
	"time"
)

func TestRepoManagedFixtureCannotBeEditedThroughProductPath(t *testing.T) {
	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	repoFixture := ProductManagedFixture{
		FixtureID:        "fixture_repo_schedule",
		TenantID:         "ten_eval",
		DisplayName:      "Repo Fixture",
		DomainClass:      FixtureDomainSchedule,
		SourceKind:       string(SourceKindFixture),
		ReviewState:      ProductStatusApproved,
		SuppressionState: SuppressionStateNone,
		RetentionState:   RetentionStateActive,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := EnsureProductFixtureEditable(repoFixture); !errors.Is(err, ErrEvaluationRepoFixtureImmutable) {
		t.Fatalf("err=%v, want repo fixture immutable", err)
	}
	if err := RejectRepoManagedFixtureEdit("repo_fixture"); !errors.Is(err, ErrEvaluationRepoFixtureImmutable) {
		t.Fatalf("err=%v, want repo fixture immutable", err)
	}
	if _, _, err := CreateProductFixtureRevision(repoFixture, FixtureRevisionInput{FixturePayload: map[string]any{}}, 2, now); !errors.Is(err, ErrEvaluationRepoFixtureImmutable) {
		t.Fatalf("err=%v, want repo fixture immutable", err)
	}
}
