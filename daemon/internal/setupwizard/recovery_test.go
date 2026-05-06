package setupwizard

import (
	"context"
	"testing"
)

func TestServiceRecoveryTransitionsPreserveCurrentSessionAndAttemptHistory(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(ServiceDependencies{Store: store})
	actor := setupActor("ten_recovery")

	session, err := service.Start(context.Background(), StartInput{TenantContext: actor, TargetID: TargetOpenAICompatible, SetupStyle: SetupStyleSubmittedSecret, Source: "wizard"})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	session, err = service.Cancel(context.Background(), ReplaceInput{TenantContext: actor, SessionID: session.SetupSessionID})
	if err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	if session.State != StateCancelled || session.ReasonCode != ReasonUserCancelled {
		t.Fatalf("cancelled session = %+v", session)
	}
	session, err = service.Retry(context.Background(), ReplaceInput{TenantContext: actor, SessionID: session.SetupSessionID})
	if err != nil {
		t.Fatalf("Retry returned error: %v", err)
	}
	if session.State != StateInProgress || session.ReasonCode != "" {
		t.Fatalf("retried session = %+v", session)
	}
	session, err = service.Replace(context.Background(), ReplaceInput{TenantContext: actor, SessionID: session.SetupSessionID})
	if err != nil {
		t.Fatalf("Replace returned error: %v", err)
	}
	if session.State != StateInProgress {
		t.Fatalf("replaced session = %+v", session)
	}
	session, err = service.Disable(context.Background(), DisableInput{TenantContext: actor, SessionID: session.SetupSessionID, DisabledReason: "operator_request"})
	if err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}
	if session.State != StateDisabled || session.SafeUseMode != SafeUseBlocked || session.ReasonCode != ReasonDisabledByUser {
		t.Fatalf("disabled session = %+v", session)
	}

	attempts, err := store.ListSetupAttempts(context.Background(), actor.TenantID, session.SetupSessionID)
	if err != nil {
		t.Fatalf("ListSetupAttempts returned error: %v", err)
	}
	if len(attempts) != 5 {
		t.Fatalf("attempt count=%d, want start+cancel+retry+replace+disable", len(attempts))
	}
	got, err := service.Get(context.Background(), actor, session.SetupSessionID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.CurrentAttemptID != attempts[len(attempts)-1].AttemptID || got.State != StateDisabled {
		t.Fatalf("current session did not converge on latest transition: got=%+v attempts=%+v", got, attempts)
	}
}
