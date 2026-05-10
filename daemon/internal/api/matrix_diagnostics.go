package api

import (
	"time"

	matrixconnector "github.com/dopejs/dope-agent/daemon/internal/connectors/matrix"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type MatrixSupportInspectionProjection struct {
	TenantID              string `json:"tenantId,omitempty"`
	ConnectorID           string `json:"connectorId"`
	TerminalState         string `json:"terminalState"`
	LatestMatrixCondition string `json:"latestMatrixCondition"`
	FreshnessState        string `json:"freshnessState"`
	InspectionElapsedMs   int64  `json:"inspectionElapsedMs"`
	RedactionStatus       string `json:"redactionStatus"`
}

func projectMatrixSupportInspection(setup store.MatrixHostedSetupRecord, condition matrixconnector.MatrixCondition, requestedAt, completedAt time.Time) MatrixSupportInspectionProjection {
	if requestedAt.IsZero() {
		requestedAt = completedAt
	}
	elapsed := completedAt.Sub(requestedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	freshness := "fresh"
	if elapsed > 2*time.Minute {
		freshness = "stale"
	}
	return MatrixSupportInspectionProjection{
		TenantID:              setup.TenantID,
		ConnectorID:           setup.ConnectorID,
		TerminalState:         setup.TerminalState,
		LatestMatrixCondition: string(condition),
		FreshnessState:        freshness,
		InspectionElapsedMs:   elapsed.Milliseconds(),
		RedactionStatus:       setup.RedactionStatus,
	}
}
