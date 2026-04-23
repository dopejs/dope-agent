package calendar

import "time"

func EventArtifact(operation Operation, item Event) Artifact {
	startsAt := item.StartsAt
	endsAt := item.EndsAt
	return Artifact{
		ArtifactID:              newID("calendar_artifact"),
		OperationID:             operation.OperationID,
		Kind:                    ArtifactKindEventSnapshot,
		IntegrationID:           operation.IntegrationID,
		EnvironmentScope:        operation.EnvironmentScope,
		ExternalEventID:         item.ExternalEventID,
		CalendarRef:             item.CalendarRef,
		Title:                   item.Title,
		StartsAt:                &startsAt,
		EndsAt:                  &endsAt,
		Timezone:                item.Timezone,
		AllDay:                  item.AllDay,
		RecurrenceSummary:       item.RecurrenceSummary,
		MutationEligibleInPhase: item.MutationEligibleInPhase,
		LifecycleState:          item.LifecycleState,
		CreatedAt:               time.Now().UTC(),
	}
}

func AvailabilityArtifact(operation Operation, query AvailabilityQuery) Artifact {
	return Artifact{
		ArtifactID:         newID("calendar_artifact"),
		OperationID:        operation.OperationID,
		Kind:               ArtifactKindAvailabilityQuery,
		IntegrationID:      operation.IntegrationID,
		EnvironmentScope:   operation.EnvironmentScope,
		CalendarRef:        operation.CalendarRef,
		Timezone:           query.Timezone,
		AvailabilityQuery:  &query,
		CreatedAt:          time.Now().UTC(),
	}
}
