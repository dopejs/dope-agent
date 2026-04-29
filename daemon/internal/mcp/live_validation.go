package mcp

import "github.com/dopejs/dope-agent/daemon/internal/livevalidation"

func LiveValidationMatrixRows() []livevalidation.MatrixRow {
	row, ok := livevalidation.DefaultMatrixRow(livevalidation.ToolClassMCPToolCall)
	if !ok {
		return nil
	}
	return []livevalidation.MatrixRow{row}
}
