package adapterrpc

import (
	"reflect"
	"strings"
	"testing"
)

// US6 / FR-003 / FR-012: the RPC envelopes must not carry ledger, idempotency, or
// side-effect-evidence state — that truth is daemon-owned. This structural test guards
// against an adapter contract drifting into a second execution plane.
func TestEnvelopesCarryNoLedgerState(t *testing.T) {
	// "operation" is intentionally NOT forbidden: Request.Operation is the op name, not ledger
	// state. These tokens denote daemon-owned ledger/evidence state an adapter must never carry.
	forbidden := []string{"ledger", "idempotency", "evidence", "artifact", "commit"}
	for _, typ := range []reflect.Type{reflect.TypeOf(Request{}), reflect.TypeOf(Response{})} {
		for i := 0; i < typ.NumField(); i++ {
			name := strings.ToLower(typ.Field(i).Name)
			for _, f := range forbidden {
				if strings.Contains(name, f) {
					t.Errorf("%s has forbidden ledger-state field %q (adapters must not own the ledger)", typ.Name(), typ.Field(i).Name)
				}
			}
		}
	}
}
