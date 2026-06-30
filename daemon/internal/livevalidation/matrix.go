package livevalidation

import (
	"errors"
	"fmt"
	"sort"
)

type ToolClass string

const (
	ToolClassDaemonInspectionRead       ToolClass = "daemon.inspection.read"
	ToolClassRuntimeLocalToolCall       ToolClass = "runtime.local_tool_call"
	ToolClassMCPToolCall                ToolClass = "mcp.tool_call"
	ToolClassIntegrationProbeRead       ToolClass = "integration.probe.read"
	ToolClassIntegrationProbeMutation   ToolClass = "integration.probe.mutation"
	ToolClassCalendarEventCreate        ToolClass = "calendar.event.create"
	ToolClassCalendarEventUpdate        ToolClass = "calendar.event.update"
	ToolClassCalendarEventCancel        ToolClass = "calendar.event.cancel"
	ToolClassCalendarAttendeeUpdate     ToolClass = "calendar.attendee.update"
	ToolClassMailDraftCreate            ToolClass = "mail.draft.create"
	ToolClassMailDraftUpdate            ToolClass = "mail.draft.update"
	ToolClassMailSend                   ToolClass = "mail.send"
	ToolClassMailReply                  ToolClass = "mail.reply"
	ToolClassMailForward                ToolClass = "mail.forward"
	ToolClassReminderLifecycleMutation  ToolClass = "reminder.lifecycle.mutation"
	ToolClassDeliveryDispatch           ToolClass = "delivery.dispatch"
	ToolClassConnectorMessageSend       ToolClass = "connector.message.send"
	ToolClassProviderSandboxUnsupported ToolClass = "provider.sandbox.unsupported"
)

type SafetyClass string

const (
	SafetyClassReadOnly              SafetyClass = "read_only"
	SafetyClassIdempotentMutation    SafetyClass = "idempotent_mutation"
	SafetyClassNonIdempotentMutation SafetyClass = "non_idempotent_mutation"
	SafetyClassUnsupported           SafetyClass = "unsupported"
)

type MatrixApproval string

const (
	MatrixApprovalNotRequired MatrixApproval = "not_required"
	MatrixApprovalScopeLevel  MatrixApproval = "scope_level"
	MatrixApprovalPerAction   MatrixApproval = "per_action"
	MatrixApprovalUnsupported MatrixApproval = "unsupported"
)

type RetryPolicy string

const (
	RetryPolicyAutomatic RetryPolicy = "automatic_retry"
	RetryPolicyManual    RetryPolicy = "manual_retry"
	RetryPolicyNone      RetryPolicy = "no_retry"
)

type CompensationKind string

const (
	CompensationNotApplicable      CompensationKind = "not_applicable"
	CompensationAutomatic          CompensationKind = "automatic_compensation"
	CompensationManualConfirmation CompensationKind = "manual_confirmation"
	CompensationUnsupported        CompensationKind = "unsupported"
)

var (
	ErrMatrixRowMissing            = errors.New("live validation support matrix row missing")
	ErrMatrixRowUnsupported        = errors.New("live validation support matrix row unsupported")
	ErrMatrixRowInvalid            = errors.New("live validation support matrix row invalid")
	ErrUnsafeAutomaticRetry        = errors.New("non-idempotent matrix row cannot allow automatic retry")
	ErrMatrixRowMissingTest        = errors.New("live validation support matrix row missing proving test")
	ErrMatrixMissingLedgerOutcomes = errors.New("live validation support matrix row missing ledger outcomes")
)

type MatrixRow struct {
	ToolClass               ToolClass        `json:"toolClass"`
	SafetyClass             SafetyClass      `json:"safetyClass"`
	Permission              string           `json:"permission"`
	Approval                MatrixApproval   `json:"approval"`
	ApprovalAction          string           `json:"approvalAction,omitempty"`
	Idempotency             string           `json:"idempotency"`
	RetryPolicy             RetryPolicy      `json:"retryPolicy"`
	AmbiguousCommitBehavior string           `json:"ambiguousCommitBehavior"`
	Compensation            CompensationKind `json:"compensation"`
	LedgerEvents            []LedgerOutcome  `json:"ledgerEvents"`
	TestCase                string           `json:"testCase"`
	Version                 string           `json:"version"`
}

func (r MatrixRow) Validate() error {
	if r.ToolClass == "" || r.SafetyClass == "" || r.RetryPolicy == "" {
		return ErrMatrixRowInvalid
	}
	if r.SafetyClass == SafetyClassUnsupported {
		if r.Approval != MatrixApprovalUnsupported && r.Compensation != CompensationUnsupported {
			return fmt.Errorf("%w: unsupported rows must not declare runnable approval or compensation", ErrMatrixRowInvalid)
		}
	} else if r.Permission == "" {
		return fmt.Errorf("%w: missing permission", ErrMatrixRowInvalid)
	}
	if r.SafetyClass == SafetyClassNonIdempotentMutation && r.RetryPolicy == RetryPolicyAutomatic {
		return ErrUnsafeAutomaticRetry
	}
	if len(r.LedgerEvents) == 0 {
		return ErrMatrixMissingLedgerOutcomes
	}
	if r.TestCase == "" {
		return ErrMatrixRowMissingTest
	}
	return nil
}

type Matrix struct {
	rows map[ToolClass]MatrixRow
}

func NewMatrix(rows []MatrixRow) (Matrix, error) {
	matrix := Matrix{rows: make(map[ToolClass]MatrixRow, len(rows))}
	for _, row := range rows {
		if err := row.Validate(); err != nil {
			return Matrix{}, fmt.Errorf("validate matrix row %s: %w", row.ToolClass, err)
		}
		matrix.rows[row.ToolClass] = row
	}
	return matrix, nil
}

func (m Matrix) Lookup(toolClass ToolClass) (MatrixRow, error) {
	row, ok := m.rows[toolClass]
	if !ok {
		return MatrixRow{}, ErrMatrixRowMissing
	}
	if row.SafetyClass == SafetyClassUnsupported {
		return row, ErrMatrixRowUnsupported
	}
	return row, nil
}

func (m Matrix) Row(toolClass ToolClass) (MatrixRow, bool) {
	row, ok := m.rows[toolClass]
	return row, ok
}

func (m Matrix) Rows() []MatrixRow {
	rows := make([]MatrixRow, 0, len(m.rows))
	for _, row := range m.rows {
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].ToolClass < rows[j].ToolClass
	})
	return rows
}

func DefaultMatrixRow(toolClass ToolClass) (MatrixRow, bool) {
	for _, row := range DefaultMatrixRows() {
		if row.ToolClass == toolClass {
			return row, true
		}
	}
	return MatrixRow{}, false
}

func DefaultMatrixRows() []MatrixRow {
	return []MatrixRow{
		supportedRow(ToolClassDaemonInspectionRead, SafetyClassReadOnly, MatrixApprovalScopeLevel, RetryPolicyAutomatic, CompensationNotApplicable, "matrix completeness and read-only replay test"),
		supportedRow(ToolClassRuntimeLocalToolCall, SafetyClassIdempotentMutation, MatrixApprovalScopeLevel, RetryPolicyAutomatic, CompensationManualConfirmation, "fake runtime local tool replay/restart test"),
		unsupportedRow(ToolClassMCPToolCall, "MCP unsupported completeness test"),
		supportedRow(ToolClassIntegrationProbeRead, SafetyClassReadOnly, MatrixApprovalScopeLevel, RetryPolicyAutomatic, CompensationNotApplicable, "fake integration read probe test"),
		supportedRow(ToolClassIntegrationProbeMutation, SafetyClassIdempotentMutation, MatrixApprovalScopeLevel, RetryPolicyManual, CompensationManualConfirmation, "fake integration mutation timeout/retry test"),
		supportedRow(ToolClassCalendarEventCreate, SafetyClassNonIdempotentMutation, MatrixApprovalPerAction, RetryPolicyNone, CompensationManualConfirmation, "fake calendar create ambiguous commit test"),
		supportedRow(ToolClassCalendarEventUpdate, SafetyClassIdempotentMutation, MatrixApprovalScopeLevel, RetryPolicyManual, CompensationManualConfirmation, "fake calendar update retry/reconciliation test"),
		supportedRow(ToolClassCalendarEventCancel, SafetyClassNonIdempotentMutation, MatrixApprovalPerAction, RetryPolicyNone, CompensationManualConfirmation, "fake calendar cancel submit-unknown test"),
		supportedRow(ToolClassCalendarAttendeeUpdate, SafetyClassNonIdempotentMutation, MatrixApprovalPerAction, RetryPolicyNone, CompensationManualConfirmation, "fake calendar attendee invitation per-action approval test"),
		supportedRow(ToolClassMailDraftCreate, SafetyClassIdempotentMutation, MatrixApprovalScopeLevel, RetryPolicyManual, CompensationManualConfirmation, "fake mail draft create test"),
		supportedRow(ToolClassMailDraftUpdate, SafetyClassIdempotentMutation, MatrixApprovalScopeLevel, RetryPolicyManual, CompensationManualConfirmation, "fake mail draft update test"),
		supportedRow(ToolClassMailSend, SafetyClassNonIdempotentMutation, MatrixApprovalPerAction, RetryPolicyNone, CompensationManualConfirmation, "fake mail send ambiguous commit test"),
		supportedRow(ToolClassMailReply, SafetyClassNonIdempotentMutation, MatrixApprovalPerAction, RetryPolicyNone, CompensationManualConfirmation, "fake mail reply ambiguous commit test"),
		supportedRow(ToolClassMailForward, SafetyClassNonIdempotentMutation, MatrixApprovalPerAction, RetryPolicyNone, CompensationManualConfirmation, "fake mail forward ambiguous commit test"),
		supportedRow(ToolClassReminderLifecycleMutation, SafetyClassIdempotentMutation, MatrixApprovalScopeLevel, RetryPolicyAutomatic, CompensationManualConfirmation, "fake reminder lifecycle test"),
		supportedRow(ToolClassDeliveryDispatch, SafetyClassNonIdempotentMutation, MatrixApprovalPerAction, RetryPolicyNone, CompensationManualConfirmation, "fake delivery dispatch duplicate retry test"),
		supportedRow(ToolClassConnectorMessageSend, SafetyClassNonIdempotentMutation, MatrixApprovalPerAction, RetryPolicyNone, CompensationManualConfirmation, "fake connector message send test"),
		unsupportedRow(ToolClassProviderSandboxUnsupported, "unsupported provider/sandbox classification test"),
	}
}

func supportedRow(toolClass ToolClass, safetyClass SafetyClass, approval MatrixApproval, retry RetryPolicy, compensation CompensationKind, testCase string) MatrixRow {
	return MatrixRow{
		ToolClass:               toolClass,
		SafetyClass:             safetyClass,
		Permission:              "live_validation.execute",
		Approval:                approval,
		ApprovalAction:          "live_validation.approve",
		Idempotency:             "validation correlation key",
		RetryPolicy:             retry,
		AmbiguousCommitBehavior: string(LedgerOutcomeOperatorActionNeeded),
		Compensation:            compensation,
		LedgerEvents:            []LedgerOutcome{LedgerOutcomeAttempted, LedgerOutcomeCompleted, LedgerOutcomeFailed, LedgerOutcomeAborted, LedgerOutcomeDenied, LedgerOutcomeOperatorActionNeeded},
		TestCase:                testCase,
		Version:                 "v1",
	}
}

func unsupportedRow(toolClass ToolClass, testCase string) MatrixRow {
	return MatrixRow{
		ToolClass:               toolClass,
		SafetyClass:             SafetyClassUnsupported,
		Approval:                MatrixApprovalUnsupported,
		Idempotency:             "not available",
		RetryPolicy:             RetryPolicyNone,
		AmbiguousCommitBehavior: "unsupported validation state",
		Compensation:            CompensationUnsupported,
		LedgerEvents:            []LedgerOutcome{LedgerOutcomeSkipped, LedgerOutcomeDenied},
		TestCase:                testCase,
		Version:                 "v1",
	}
}
