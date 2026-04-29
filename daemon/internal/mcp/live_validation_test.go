package mcp

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

func TestMCPToolCallsAreUnsupportedByDefaultForLiveValidation(t *testing.T) {
	rows := LiveValidationMatrixRows()
	if len(rows) != 1 {
		t.Fatalf("len(rows)=%d, want 1", len(rows))
	}
	row := rows[0]
	if row.ToolClass != livevalidation.ToolClassMCPToolCall || row.SafetyClass != livevalidation.SafetyClassUnsupported {
		t.Fatalf("unexpected MCP classification: %+v", row)
	}
	if row.RetryPolicy != livevalidation.RetryPolicyNone || row.Compensation != livevalidation.CompensationUnsupported {
		t.Fatalf("MCP unsupported row allows retry or compensation: %+v", row)
	}
}
