package auth

import (
	"errors"
	"testing"
	"time"
)

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

func TestAuthenticateRejectsRevokedExpiredAndRotatedTokens(t *testing.T) {
	manager := NewManager()
	pairing, code, err := manager.StartPairing(StartPairingInput{Mode: PairingModeLocal, Label: "cli"})
	if err != nil {
		t.Fatalf("StartPairing returned error: %v", err)
	}
	_, token, tokenSecret, err := manager.CompletePairing(pairing.PairingID, CompletePairingInput{Code: code})
	if err != nil {
		t.Fatalf("CompletePairing returned error: %v", err)
	}

	for _, tt := range []struct {
		status string
		want   error
	}{
		{status: "revoked", want: ErrTokenRevoked},
		{status: "expired", want: ErrTokenExpired},
		{status: "rotated", want: ErrTokenRotated},
	} {
		manager.Restore(nil, []AccessToken{{
			TokenID:      token.TokenID,
			Label:        token.Label,
			Mode:         token.Mode,
			TokenHash:    token.TokenHash,
			TokenPreview: token.TokenPreview,
			Status:       tt.status,
			CreatedAt:    token.CreatedAt,
			UpdatedAt:    token.UpdatedAt,
		}})
		if _, err := manager.Authenticate(tokenSecret); err != tt.want {
			t.Fatalf("expected %v for status %s, got %v", tt.want, tt.status, err)
		}
	}
}

func TestIssueRevokeAndRotateTokenLifecycle(t *testing.T) {
	manager := NewManager()
	expiresAt := time.Now().UTC().Add(time.Hour)

	token, secret, err := manager.IssueToken(IssueTokenInput{
		PrincipalID:     "prn_1",
		Label:           "automation",
		DefaultTenantID: "ten_1",
		ExpiresAt:       &expiresAt,
	})
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}
	if token.TokenID == "" || secret == "" || token.TokenHash == "" || token.TokenHash == secret {
		t.Fatalf("expected hashed token material and one-time secret, token=%+v secret=%q", token, secret)
	}
	if token.PrincipalID != "prn_1" || token.DefaultTenantID != "ten_1" || token.Status != "active" {
		t.Fatalf("unexpected issued token: %+v", token)
	}
	if _, err := manager.Authenticate(secret); err != nil {
		t.Fatalf("Authenticate issued token returned error: %v", err)
	}

	revoked, err := manager.RevokeToken(token.TokenID)
	if err != nil {
		t.Fatalf("RevokeToken returned error: %v", err)
	}
	if revoked.Status != "revoked" || revoked.RevokedAt == nil {
		t.Fatalf("expected revoked lifecycle, got %+v", revoked)
	}
	if _, err := manager.Authenticate(secret); !errors.Is(err, ErrTokenRevoked) {
		t.Fatalf("expected revoked token denial, got %v", err)
	}
}

func TestRotateTokenPreservesAuthorityLineage(t *testing.T) {
	manager := NewManager()
	oldToken, oldSecret, err := manager.IssueToken(IssueTokenInput{
		PrincipalID:     "prn_1",
		Label:           "automation",
		DefaultTenantID: "ten_1",
	})
	if err != nil {
		t.Fatalf("IssueToken returned error: %v", err)
	}

	rotated, replacement, replacementSecret, err := manager.RotateToken(oldToken.TokenID, RotateTokenInput{Reason: "scheduled"})
	if err != nil {
		t.Fatalf("RotateToken returned error: %v", err)
	}
	if rotated.Status != "rotated" || rotated.RotatedToTokenID != replacement.TokenID {
		t.Fatalf("unexpected rotated old token: %+v", rotated)
	}
	if replacement.RotatedFromTokenID != oldToken.TokenID || replacement.PrincipalID != oldToken.PrincipalID || replacement.DefaultTenantID != oldToken.DefaultTenantID {
		t.Fatalf("unexpected replacement lineage: old=%+v new=%+v", rotated, replacement)
	}
	if _, err := manager.Authenticate(oldSecret); !errors.Is(err, ErrTokenRotated) {
		t.Fatalf("expected old token rotated denial, got %v", err)
	}
	if _, err := manager.Authenticate(replacementSecret); err != nil {
		t.Fatalf("Authenticate replacement token returned error: %v", err)
	}
}
