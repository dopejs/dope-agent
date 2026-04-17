package router

import "testing"

func TestRouteReusesDirectSessionAndIsolatesGroupSessions(t *testing.T) {
	r := NewSessionRouter()

	direct, created, err := r.Route(RouteInput{
		Kind:      SessionKindDirect,
		Channel:   "telegram",
		AccountID: "acct_1",
		PeerID:    "user_1",
	})
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if !created {
		t.Fatal("expected first direct session to be created")
	}

	sameDirect, created, err := r.Route(RouteInput{
		Kind:      SessionKindDirect,
		Channel:   "telegram",
		AccountID: "acct_1",
		PeerID:    "user_1",
	})
	if err != nil {
		t.Fatalf("Route returned error on second direct route: %v", err)
	}
	if created {
		t.Fatal("expected direct route to reuse session")
	}
	if sameDirect.SessionID != direct.SessionID {
		t.Fatalf("expected same direct session ID %s, got %s", direct.SessionID, sameDirect.SessionID)
	}

	groupA, created, err := r.Route(RouteInput{
		Kind:      SessionKindGroup,
		Channel:   "telegram",
		AccountID: "acct_1",
		PeerID:    "group_1",
		ThreadID:  "thread_a",
	})
	if err != nil {
		t.Fatalf("Route returned error for group A: %v", err)
	}
	if !created {
		t.Fatal("expected group session A to be created")
	}

	groupB, created, err := r.Route(RouteInput{
		Kind:      SessionKindGroup,
		Channel:   "telegram",
		AccountID: "acct_1",
		PeerID:    "group_1",
		ThreadID:  "thread_b",
	})
	if err != nil {
		t.Fatalf("Route returned error for group B: %v", err)
	}
	if !created {
		t.Fatal("expected group session B to be created")
	}
	if groupA.SessionID == groupB.SessionID {
		t.Fatal("expected different group sessions for different thread IDs")
	}
}

func TestResetSessionIncrementsGeneration(t *testing.T) {
	r := NewSessionRouter()

	session, _, err := r.Route(RouteInput{
		Channel: "local",
		PeerID:  "chat",
	})
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}

	reset, err := r.ResetSession(session.SessionID)
	if err != nil {
		t.Fatalf("ResetSession returned error: %v", err)
	}
	if reset.Generation != 2 {
		t.Fatalf("expected generation 2 after reset, got %d", reset.Generation)
	}
	if reset.LastResetAt == nil {
		t.Fatal("expected LastResetAt to be set")
	}
}
