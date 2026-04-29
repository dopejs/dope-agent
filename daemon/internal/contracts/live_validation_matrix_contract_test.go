package contracts

import (
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/livevalidation"
)

func TestLiveValidationSupportMatrixCoversRequiredToolClasses(t *testing.T) {
	required := []livevalidation.ToolClass{
		livevalidation.ToolClassDaemonInspectionRead,
		livevalidation.ToolClassRuntimeLocalToolCall,
		livevalidation.ToolClassMCPToolCall,
		livevalidation.ToolClassIntegrationProbeRead,
		livevalidation.ToolClassIntegrationProbeMutation,
		livevalidation.ToolClassCalendarEventCreate,
		livevalidation.ToolClassCalendarEventUpdate,
		livevalidation.ToolClassCalendarEventCancel,
		livevalidation.ToolClassMailDraftCreate,
		livevalidation.ToolClassMailDraftUpdate,
		livevalidation.ToolClassMailSend,
		livevalidation.ToolClassMailReply,
		livevalidation.ToolClassMailForward,
		livevalidation.ToolClassReminderLifecycleMutation,
		livevalidation.ToolClassDeliveryDispatch,
		livevalidation.ToolClassConnectorMessageSend,
		livevalidation.ToolClassProviderSandboxUnsupported,
	}
	matrix, err := livevalidation.NewMatrix(livevalidation.DefaultMatrixRows())
	if err != nil {
		t.Fatalf("NewMatrix returned error: %v", err)
	}
	for _, toolClass := range required {
		row, ok := matrix.Row(toolClass)
		if !ok {
			t.Fatalf("required tool class %s has no matrix row", toolClass)
		}
		if row.TestCase == "" || len(row.LedgerEvents) == 0 {
			t.Fatalf("required tool class %s has incomplete contract row: %+v", toolClass, row)
		}
	}
}
