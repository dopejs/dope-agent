package adapterref

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterrpc"
)

// US3 / FR-006: the reference adapter receives per-call credentials but must not echo or
// otherwise surface them in its response.
func TestHandleDoesNotLeakCredential(t *testing.T) {
	resp := Handle(adapterrpc.Request{
		RequestID:  "r1",
		Operation:  "ProjectAccount",
		Credential: json.RawMessage(`"top-secret-material"`),
	})
	blob, _ := json.Marshal(resp)
	if strings.Contains(string(blob), "top-secret-material") {
		t.Fatalf("reference adapter response leaked credential material: %s", blob)
	}
}
