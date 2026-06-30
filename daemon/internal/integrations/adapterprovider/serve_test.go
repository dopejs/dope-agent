package adapterprovider_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterprovider"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterrpc"
)

type handlerFunc func(ctx context.Context, op adapterprovider.Operation) (json.RawMessage, error)

func (f handlerFunc) Handle(ctx context.Context, op adapterprovider.Operation) (json.RawMessage, error) {
	return f(ctx, op)
}

// pipeClient wires a Handler behind Serve over pipes and returns a connected RPC client.
func pipeClient(t *testing.T, h adapterprovider.Handler) *adapterrpc.Client {
	t.Helper()
	adapterReader, daemonWriter := io.Pipe() // daemon -> adapter
	daemonReader, adapterWriter := io.Pipe() // adapter -> daemon
	go func() { _ = adapterprovider.Serve(adapterReader, adapterWriter, h); _ = adapterWriter.Close() }()
	t.Cleanup(func() { _ = daemonWriter.Close(); _ = adapterWriter.Close() })
	return adapterrpc.NewClient(daemonWriter, daemonReader)
}

func TestServeReadyHandshake(t *testing.T) {
	client := pipeClient(t, handlerFunc(func(ctx context.Context, op adapterprovider.Operation) (json.RawMessage, error) {
		t.Fatal("Ready must be answered locally, not dispatched to the handler")
		return nil, nil
	}))
	if err := client.Ready(context.Background()); err != nil {
		t.Fatalf("Ready: %v", err)
	}
}

func TestServeOKAndFault(t *testing.T) {
	client := pipeClient(t, handlerFunc(func(ctx context.Context, op adapterprovider.Operation) (json.RawMessage, error) {
		if op.Operation == "Fail" {
			return nil, &adapterprovider.Fault{Kind: adapterrpc.FailureScope, Code: "scope_not_granted", Message: "denied"}
		}
		return json.RawMessage(`{"ok":true}`), nil
	}))

	var out map[string]bool
	if err := client.Dispatch(context.Background(), "calendar", "Read", nil, nil, &out); err != nil {
		t.Fatalf("Dispatch ok: %v", err)
	}
	if !out["ok"] {
		t.Fatalf("payload not returned: %+v", out)
	}

	err := client.Dispatch(context.Background(), "calendar", "Fail", nil, nil, nil)
	ae, ok := err.(*adapterrpc.AdapterError)
	if !ok {
		t.Fatalf("error = %T, want *adapterrpc.AdapterError", err)
	}
	if ae.Kind != adapterrpc.FailureScope || ae.Detail != "scope_not_granted" {
		t.Fatalf("adapter error = %+v", ae)
	}
}

func TestServeAmbiguousBecomesUndecodable(t *testing.T) {
	client := pipeClient(t, handlerFunc(func(ctx context.Context, op adapterprovider.Operation) (json.RawMessage, error) {
		return nil, adapterprovider.ErrAmbiguous
	}))
	err := client.Dispatch(context.Background(), "calendar", "Write", nil, nil, nil)
	if !adapterrpc.IsAmbiguous(err) {
		t.Fatalf("err = %v, want ambiguous-commit (undecodable) classification", err)
	}
}
