package livevalidation

import (
	"errors"
	"testing"
)

func TestMatrixRowsValidateRequiredColumns(t *testing.T) {
	matrix, err := NewMatrix(DefaultMatrixRows())
	if err != nil {
		t.Fatalf("NewMatrix(DefaultMatrixRows) returned error: %v", err)
	}
	for _, row := range matrix.Rows() {
		if row.ToolClass == "" || row.SafetyClass == "" || row.Approval == "" || row.Idempotency == "" || row.RetryPolicy == "" || row.AmbiguousCommitBehavior == "" || row.Compensation == "" || row.TestCase == "" || row.Version == "" {
			t.Fatalf("matrix row has missing required columns: %+v", row)
		}
		if row.SafetyClass != SafetyClassUnsupported && row.Permission != "live_validation.execute" {
			t.Fatalf("supported row %s missing execute permission: %+v", row.ToolClass, row)
		}
		if len(row.LedgerEvents) == 0 {
			t.Fatalf("matrix row %s missing ledger events", row.ToolClass)
		}
	}
}

func TestMatrixMissingRowsAreUnsupported(t *testing.T) {
	matrix, err := NewMatrix(DefaultMatrixRows())
	if err != nil {
		t.Fatalf("NewMatrix returned error: %v", err)
	}
	if _, err := matrix.Lookup(ToolClass("unknown.future_tool")); !errors.Is(err, ErrMatrixRowMissing) {
		t.Fatalf("Lookup missing row err=%v, want ErrMatrixRowMissing", err)
	}
	if _, err := matrix.Lookup(ToolClassMCPToolCall); !errors.Is(err, ErrMatrixRowUnsupported) {
		t.Fatalf("Lookup unsupported row err=%v, want ErrMatrixRowUnsupported", err)
	}
}

func TestMatrixRejectsUnsafeNonIdempotentAutomaticRetry(t *testing.T) {
	row := supportedRow(ToolClassMailSend, SafetyClassNonIdempotentMutation, MatrixApprovalPerAction, RetryPolicyAutomatic, CompensationManualConfirmation, "bad retry test")
	if err := row.Validate(); !errors.Is(err, ErrUnsafeAutomaticRetry) {
		t.Fatalf("Validate err=%v, want ErrUnsafeAutomaticRetry", err)
	}
}
