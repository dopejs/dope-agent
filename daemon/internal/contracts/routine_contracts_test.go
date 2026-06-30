package contracts_test

import (
	"testing"

	contracts "github.com/dopejs/dope-agent/daemon/internal/contracts"
)

func TestRoutineSchemasAcceptCanonicalFixtures(t *testing.T) {
	t.Parallel()

	validator := contracts.NewValidator(schemaRootDir(t))
	fixtures := map[string]string{
		"schemas/api/create-routine.request.schema.json": `{"definition":{"name":"Daily summary","trigger":{"kind":"cron","cronExpr":"0 8 * * *","timezone":"UTC"},"workflow":{"goal":"summarize my day"},"approvalExpectation":"ask","maxRetries":1}}`,
		"schemas/api/routine-resource.schema.json":       `{"routineId":"routine_1","environmentScope":"test","name":"Daily summary","state":"active","currentVersion":1,"currentScheduleId":"sched_1","definition":{"name":"Daily summary","trigger":{"kind":"cron","cronExpr":"0 8 * * *","timezone":"UTC"},"workflow":{"entrypoint":"operator","goal":"summarize my day"},"approvalExpectation":"ask","maxRetries":1},"versions":[{"version":1,"definition":{"name":"Daily summary","trigger":{"kind":"cron","cronExpr":"0 8 * * *"},"workflow":{"goal":"summarize my day"}},"scheduleId":"sched_1","createdAt":"2026-06-30T10:00:00Z"}],"createdAt":"2026-06-30T10:00:00Z","updatedAt":"2026-06-30T10:00:00Z"}`,
	}
	for schema, fixture := range fixtures {
		if err := validator.ValidateRelative(schema, []byte(fixture)); err != nil {
			t.Fatalf("ValidateRelative(%s): %v", schema, err)
		}
	}
}
