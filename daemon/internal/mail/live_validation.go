package mail

import "github.com/dopejs/dope-agent/daemon/internal/livevalidation"

func LiveValidationMatrixRows() []livevalidation.MatrixRow {
	classes := []livevalidation.ToolClass{
		livevalidation.ToolClassMailDraftCreate,
		livevalidation.ToolClassMailDraftUpdate,
		livevalidation.ToolClassMailSend,
		livevalidation.ToolClassMailReply,
		livevalidation.ToolClassMailForward,
	}
	rows := make([]livevalidation.MatrixRow, 0, len(classes))
	for _, toolClass := range classes {
		if row, ok := livevalidation.DefaultMatrixRow(toolClass); ok {
			rows = append(rows, row)
		}
	}
	return rows
}
