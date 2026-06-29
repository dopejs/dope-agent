package adapterrpc

import "context"

// conformanceOps is the representative operation set every integration adapter must satisfy
// to be conformant. It mirrors the calendar and mail Backend interfaces.
var conformanceOps = []struct {
	Domain    string
	Operation string
}{
	{"calendar", "ProjectAccount"},
	{"calendar", "ListEvents"},
	{"calendar", "GetEvent"},
	{"calendar", "BusyFree"},
	{"calendar", "CreateEvent"},
	{"calendar", "UpdateEvent"},
	{"calendar", "CancelEvent"},
	{"mail", "ProjectAccount"},
	{"mail", "ListThreads"},
	{"mail", "GetThread"},
	{"mail", "GetMessage"},
	{"mail", "ListDrafts"},
	{"mail", "GetDraft"},
	{"mail", "CreateDraft"},
	{"mail", "UpdateDraft"},
	{"mail", "SendMessage"},
	{"mail", "SendDraft"},
	{"mail", "ReplyMessage"},
	{"mail", "ForwardMessage"},
	{"mail", "ResolveAttachments"},
}

// ConformanceResult is the outcome for one operation.
type ConformanceResult struct {
	Domain    string
	Operation string
	OK        bool
	Err       error
}

// ConformanceReport is the result of running an adapter against the contract.
type ConformanceReport struct {
	ReadyErr error
	Results  []ConformanceResult
}

// Passed reports whether the adapter satisfied the readiness handshake and every operation.
func (r ConformanceReport) Passed() bool {
	if r.ReadyErr != nil || len(r.Results) == 0 {
		return false
	}
	for _, res := range r.Results {
		if !res.OK {
			return false
		}
	}
	return true
}

// RunConformance verifies an adapter (reached via client) against the contract: it performs
// the version readiness handshake, then dispatches every contract operation and records
// whether each returned a valid, contract-conformant response. A version mismatch, transport
// break, or malformed response causes the relevant step to fail (FR-009). It does not mutate
// real state — the adapter under test is expected to be the reference skeleton or a provider
// adapter pointed at a safe target.
func RunConformance(ctx context.Context, client *Client) ConformanceReport {
	report := ConformanceReport{}
	if err := client.Ready(ctx); err != nil {
		report.ReadyErr = err
		return report
	}
	for _, op := range conformanceOps {
		err := client.Dispatch(ctx, op.Domain, op.Operation, map[string]any{}, nil, nil)
		report.Results = append(report.Results, ConformanceResult{
			Domain:    op.Domain,
			Operation: op.Operation,
			OK:        err == nil,
			Err:       err,
		})
	}
	return report
}
