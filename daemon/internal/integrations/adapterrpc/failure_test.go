package adapterrpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterref"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterrpc"
)

// US2: adapter failures must be classified without crashing the daemon. Unconfirmed outcomes
// (crash, hang/timeout, malformed) are ambiguous; a clean auth failure is confirmed.

func TestCrashIsAmbiguousUnreachable(t *testing.T) {
	client, stop := adapterref.NewPipeClientWithOptions(adapterref.Options{FailMode: adapterref.FailCrash})
	defer stop()
	err := client.Dispatch(context.Background(), "calendar", "CreateEvent", nil, map[string]any{}, nil)
	if err == nil || !adapterrpc.IsAmbiguous(err) {
		t.Fatalf("crash: want ambiguous error, got %v", err)
	}
}

func TestMalformedResponseIsAmbiguous(t *testing.T) {
	client, stop := adapterref.NewPipeClientWithOptions(adapterref.Options{FailMode: adapterref.FailMalformed})
	defer stop()
	err := client.Dispatch(context.Background(), "calendar", "CreateEvent", nil, map[string]any{}, nil)
	if err == nil || !adapterrpc.IsAmbiguous(err) {
		t.Fatalf("malformed: want ambiguous error, got %v", err)
	}
}

func TestHangBeyondDeadlineIsAmbiguous(t *testing.T) {
	client, stop := adapterref.NewPipeClientWithOptions(adapterref.Options{FailMode: adapterref.FailHang, HangFor: time.Second})
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := client.Dispatch(ctx, "calendar", "CreateEvent", nil, map[string]any{}, nil)
	if err == nil || !adapterrpc.IsAmbiguous(err) {
		t.Fatalf("hang: want ambiguous error, got %v", err)
	}
}

func TestAuthFailureIsConfirmedNotAmbiguous(t *testing.T) {
	client, stop := adapterref.NewPipeClientWithOptions(adapterref.Options{FailMode: adapterref.FailAuth})
	defer stop()
	err := client.Dispatch(context.Background(), "mail", "SendMessage", nil, map[string]any{}, nil)
	if err == nil {
		t.Fatal("auth: want error")
	}
	if adapterrpc.IsAmbiguous(err) {
		t.Fatalf("auth failure must be confirmed (not ambiguous), got %v", err)
	}
	var ae *adapterrpc.AdapterError
	if !asErr(err, &ae) || ae.Kind != adapterrpc.FailureAuth {
		t.Fatalf("want AdapterError auth, got %v", err)
	}
}

func TestDispatchWithoutCallerDeadlineIsStillBounded(t *testing.T) {
	// A caller that passes context.Background() (no deadline) must still be bounded by the
	// client default, so a hung adapter cannot block forever.
	client, stop := adapterref.NewPipeClientWithOptions(adapterref.Options{FailMode: adapterref.FailHang, HangFor: 3 * time.Second})
	defer stop()
	client.WithDefaultDeadline(40 * time.Millisecond)

	done := make(chan error, 1)
	go func() { done <- client.Dispatch(context.Background(), "calendar", "CreateEvent", nil, nil, nil) }()
	select {
	case err := <-done:
		if err == nil || !adapterrpc.IsAmbiguous(err) {
			t.Fatalf("want ambiguous (timeout) error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch was not bounded by the default deadline (hung)")
	}
}

func TestTransportPoisonedAfterTimeoutFailsFast(t *testing.T) {
	client, stop := adapterref.NewPipeClientWithOptions(adapterref.Options{FailMode: adapterref.FailHang, HangFor: 3 * time.Second})
	defer stop()
	client.WithDefaultDeadline(40 * time.Millisecond)

	if err := client.Dispatch(context.Background(), "calendar", "CreateEvent", nil, nil, nil); !adapterrpc.IsAmbiguous(err) {
		t.Fatalf("first call: want ambiguous timeout, got %v", err)
	}
	// Subsequent call must fail fast (broken transport), not spawn a competing reader.
	start := time.Now()
	err := client.Dispatch(context.Background(), "calendar", "GetEvent", nil, nil, nil)
	if err != adapterrpc.ErrUnreachable {
		t.Fatalf("second call: want ErrUnreachable (poisoned), got %v", err)
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("poisoned transport did not fail fast")
	}
}

func asErr(err error, target **adapterrpc.AdapterError) bool {
	if ae, ok := err.(*adapterrpc.AdapterError); ok {
		*target = ae
		return true
	}
	return false
}
