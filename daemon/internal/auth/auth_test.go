package auth

import "testing"

func TestPairingAndAuthenticationLifecycle(t *testing.T) {
	manager := NewManager()

	pairing, code, err := manager.StartPairing(StartPairingInput{
		Mode:  PairingModeLocal,
		Label: "web-ui",
	})
	if err != nil {
		t.Fatalf("StartPairing returned error: %v", err)
	}
	if pairing.Status != PairingStatusPending {
		t.Fatalf("expected pending pairing, got %s", pairing.Status)
	}

	completedPairing, token, tokenSecret, err := manager.CompletePairing(pairing.PairingID, CompletePairingInput{Code: code})
	if err != nil {
		t.Fatalf("CompletePairing returned error: %v", err)
	}
	if completedPairing.Status != PairingStatusCompleted {
		t.Fatalf("expected completed pairing, got %s", completedPairing.Status)
	}
	if token.TokenID == "" || tokenSecret == "" {
		t.Fatal("expected token and token secret")
	}

	authenticated, err := manager.Authenticate(tokenSecret)
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if authenticated.TokenID != token.TokenID {
		t.Fatalf("expected token ID %s, got %s", token.TokenID, authenticated.TokenID)
	}
	if authenticated.LastUsedAt == nil {
		t.Fatal("expected lastUsedAt to be updated")
	}
}

func TestAuthenticateRejectsInvalidToken(t *testing.T) {
	manager := NewManager()

	if _, err := manager.Authenticate("bad-token"); err != ErrTokenInvalid {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
}
