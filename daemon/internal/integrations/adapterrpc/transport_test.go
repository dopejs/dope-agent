package adapterrpc

import (
	"bufio"
	"context"
	"io"
	"testing"
	"time"
)

// inlineAdapter runs a minimal responder over pipes, returning the response produced by
// respond() for each request. It lets transport/client behavior be tested without a process.
func inlineAdapter(t *testing.T, respond func(Request) Response) *Client {
	t.Helper()
	adapterReader, daemonWriter := io.Pipe() // daemon -> adapter
	daemonReader, adapterWriter := io.Pipe() // adapter -> daemon

	go func() {
		br := bufio.NewReader(adapterReader)
		for {
			req, err := ReadRequest(br)
			if err != nil {
				return
			}
			if werr := WriteMessage(adapterWriter, respond(req)); werr != nil {
				return
			}
		}
	}()

	c := NewClient(daemonWriter, daemonReader)
	t.Cleanup(func() { _ = daemonWriter.Close(); _ = adapterWriter.Close() })
	return c
}

func okResponse(req Request, payload string) Response {
	return Response{RequestID: req.RequestID, ContractVersion: ContractVersion, Status: StatusOK, Payload: []byte(payload)}
}

func TestDispatchRoundTrip(t *testing.T) {
	c := inlineAdapter(t, func(req Request) Response {
		if req.Domain != "calendar" || req.Operation != "ProjectAccount" {
			t.Errorf("unexpected request: %+v", req)
		}
		return okResponse(req, `{"integrationId":"int-1"}`)
	})

	var out struct {
		IntegrationID string `json:"integrationId"`
	}
	if err := c.Dispatch(context.Background(), "calendar", "ProjectAccount", nil, map[string]any{"x": 1}, &out); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if out.IntegrationID != "int-1" {
		t.Fatalf("payload not decoded, got %q", out.IntegrationID)
	}
}

func TestDispatchContractMismatch(t *testing.T) {
	c := inlineAdapter(t, func(req Request) Response {
		r := okResponse(req, `{}`)
		r.ContractVersion = "999"
		return r
	})
	err := c.Dispatch(context.Background(), "calendar", "ProjectAccount", nil, nil, nil)
	if err != ErrContractMismatch {
		t.Fatalf("want ErrContractMismatch, got %v", err)
	}
}

func TestDispatchFailureMapped(t *testing.T) {
	c := inlineAdapter(t, func(req Request) Response {
		return Response{RequestID: req.RequestID, ContractVersion: ContractVersion, Status: StatusFailure, FailureKind: FailureAuth}
	})
	err := c.Dispatch(context.Background(), "mail", "SendMessage", nil, nil, nil)
	var ae *AdapterError
	if !asAdapterError(err, &ae) || ae.Kind != FailureAuth {
		t.Fatalf("want AdapterError auth, got %v", err)
	}
}

func TestDispatchDeadline(t *testing.T) {
	c := inlineAdapter(t, func(req Request) Response {
		time.Sleep(200 * time.Millisecond)
		return okResponse(req, `{}`)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := c.Dispatch(ctx, "calendar", "ListEvents", nil, nil, nil)
	if err == nil {
		t.Fatal("want deadline error, got nil")
	}
}

func asAdapterError(err error, target **AdapterError) bool {
	if ae, ok := err.(*AdapterError); ok {
		*target = ae
		return true
	}
	return false
}
