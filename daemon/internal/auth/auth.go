package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

type PairingMode string

const (
	PairingModeLocal PairingMode = "local"
	PairingModeToken PairingMode = "token"
)

type PairingStatus string

const (
	PairingStatusPending   PairingStatus = "pending"
	PairingStatusCompleted PairingStatus = "completed"
	PairingStatusExpired   PairingStatus = "expired"
)

var (
	ErrPairingModeInvalid  = errors.New("pairing mode is invalid")
	ErrPairingNotFound     = errors.New("pairing not found")
	ErrPairingCodeInvalid  = errors.New("pairing code is invalid")
	ErrPairingNotPending   = errors.New("pairing is not pending")
	ErrTokenInvalid        = errors.New("access token is invalid")
	ErrAccessTokenNotFound = errors.New("access token not found")
	ErrAuthRequired        = errors.New("authentication is required")
	ErrTokenRevoked        = errors.New("access token is revoked")
	ErrTokenExpired        = errors.New("access token is expired")
	ErrTokenRotated        = errors.New("access token is rotated")
)

type Pairing struct {
	PairingID   string        `json:"pairingId"`
	Mode        PairingMode   `json:"mode"`
	Label       string        `json:"label"`
	Status      PairingStatus `json:"status"`
	CodeHash    string        `json:"-"`
	CodePreview string        `json:"codePreview,omitempty"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
	ExpiresAt   time.Time     `json:"expiresAt"`
	CompletedAt *time.Time    `json:"completedAt,omitempty"`
}

type AccessToken struct {
	TokenID            string      `json:"tokenId"`
	PrincipalID        string      `json:"principalId,omitempty"`
	Label              string      `json:"label"`
	Mode               PairingMode `json:"mode"`
	TokenHash          string      `json:"-"`
	TokenPreview       string      `json:"tokenPreview"`
	Status             string      `json:"status"`
	DefaultTenantID    string      `json:"defaultTenantId,omitempty"`
	CreatedAt          time.Time   `json:"createdAt"`
	UpdatedAt          time.Time   `json:"updatedAt"`
	LastUsedAt         *time.Time  `json:"lastUsedAt,omitempty"`
	ExpiresAt          *time.Time  `json:"expiresAt,omitempty"`
	RevokedAt          *time.Time  `json:"revokedAt,omitempty"`
	RotatedFromTokenID string      `json:"rotatedFromTokenId,omitempty"`
	RotatedToTokenID   string      `json:"rotatedToTokenId,omitempty"`
}

type StartPairingInput struct {
	Mode       PairingMode `json:"mode"`
	Label      string      `json:"label"`
	TTLSeconds int         `json:"ttlSeconds"`
}

type CompletePairingInput struct {
	Code string `json:"code"`
}

type IssueTokenInput struct {
	PrincipalID     string     `json:"principalId"`
	Label           string     `json:"label"`
	DefaultTenantID string     `json:"defaultTenantId"`
	ExpiresAt       *time.Time `json:"expiresAt,omitempty"`
}

type RotateTokenInput struct {
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	Reason    string     `json:"reason,omitempty"`
}

type Manager struct {
	mu         sync.RWMutex
	pairings   map[string]Pairing
	pairingIDs []string
	tokens     map[string]AccessToken
	tokenIDs   []string
}

func NewManager() *Manager {
	return &Manager{
		pairings: make(map[string]Pairing),
		tokens:   make(map[string]AccessToken),
	}
}

func (m *Manager) StartPairing(input StartPairingInput) (Pairing, string, error) {
	mode := input.Mode
	if mode == "" {
		mode = PairingModeLocal
	}
	if mode != PairingModeLocal && mode != PairingModeToken {
		return Pairing{}, "", ErrPairingModeInvalid
	}

	ttlSeconds := input.TTLSeconds
	if ttlSeconds <= 0 {
		ttlSeconds = 600
	}

	now := time.Now().UTC()
	code := randomDigits(6)
	pairing := Pairing{
		PairingID:   "pair_" + randomHex(8, "fallback"),
		Mode:        mode,
		Label:       input.Label,
		Status:      PairingStatusPending,
		CodeHash:    hashSecret(code),
		CodePreview: code,
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   now.Add(time.Duration(ttlSeconds) * time.Second),
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.pairings[pairing.PairingID] = pairing
	m.pairingIDs = append(m.pairingIDs, pairing.PairingID)

	return pairing, code, nil
}

func (m *Manager) CompletePairing(pairingID string, input CompletePairingInput) (Pairing, AccessToken, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pairing, ok := m.pairings[pairingID]
	if !ok {
		return Pairing{}, AccessToken{}, "", ErrPairingNotFound
	}
	if pairing.Status != PairingStatusPending {
		return Pairing{}, AccessToken{}, "", ErrPairingNotPending
	}
	if time.Now().UTC().After(pairing.ExpiresAt) {
		now := time.Now().UTC()
		pairing.Status = PairingStatusExpired
		pairing.UpdatedAt = now
		m.pairings[pairingID] = pairing
		return Pairing{}, AccessToken{}, "", ErrPairingNotPending
	}
	if hashSecret(input.Code) != pairing.CodeHash {
		return Pairing{}, AccessToken{}, "", ErrPairingCodeInvalid
	}

	now := time.Now().UTC()
	tokenSecret := "dope_" + randomHex(24, "fallback")
	token := AccessToken{
		TokenID:      "tok_" + randomHex(8, "fallback"),
		Label:        pairing.Label,
		Mode:         pairing.Mode,
		TokenHash:    hashSecret(tokenSecret),
		TokenPreview: tokenSecret[:12],
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	pairing.Status = PairingStatusCompleted
	pairing.UpdatedAt = now
	pairing.CompletedAt = &now
	pairing.CodePreview = ""
	m.pairings[pairingID] = pairing
	m.tokens[token.TokenID] = token
	m.tokenIDs = append(m.tokenIDs, token.TokenID)

	return pairing, token, tokenSecret, nil
}

func (m *Manager) IssueToken(input IssueTokenInput) (AccessToken, string, error) {
	if input.PrincipalID == "" || input.DefaultTenantID == "" {
		return AccessToken{}, "", ErrTokenInvalid
	}
	now := time.Now().UTC()
	tokenSecret := "dope_" + randomHex(24, "fallback")
	token := AccessToken{
		TokenID:         "tok_" + randomHex(8, "fallback"),
		PrincipalID:     input.PrincipalID,
		Label:           input.Label,
		Mode:            PairingModeToken,
		TokenHash:       hashSecret(tokenSecret),
		TokenPreview:    tokenSecret[:12],
		Status:          "active",
		DefaultTenantID: input.DefaultTenantID,
		CreatedAt:       now,
		UpdatedAt:       now,
		ExpiresAt:       input.ExpiresAt,
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[token.TokenID] = token
	m.tokenIDs = append(m.tokenIDs, token.TokenID)
	return token, tokenSecret, nil
}

func (m *Manager) RevokeToken(tokenID string) (AccessToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	token, ok := m.tokens[tokenID]
	if !ok {
		return AccessToken{}, ErrAccessTokenNotFound
	}
	if token.Status == "rotated" {
		return AccessToken{}, ErrTokenRotated
	}
	if token.Status == "expired" {
		return AccessToken{}, ErrTokenExpired
	}
	now := time.Now().UTC()
	token.Status = "revoked"
	token.UpdatedAt = now
	token.RevokedAt = &now
	m.tokens[tokenID] = token
	return token, nil
}

func (m *Manager) RotateToken(tokenID string, input RotateTokenInput) (AccessToken, AccessToken, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldToken, ok := m.tokens[tokenID]
	if !ok {
		return AccessToken{}, AccessToken{}, "", ErrAccessTokenNotFound
	}
	switch oldToken.Status {
	case "", "active":
	case "revoked":
		return AccessToken{}, AccessToken{}, "", ErrTokenRevoked
	case "expired":
		return AccessToken{}, AccessToken{}, "", ErrTokenExpired
	case "rotated":
		return AccessToken{}, AccessToken{}, "", ErrTokenRotated
	default:
		return AccessToken{}, AccessToken{}, "", ErrTokenInvalid
	}
	now := time.Now().UTC()
	if oldToken.ExpiresAt != nil && !oldToken.ExpiresAt.After(now) {
		oldToken.Status = "expired"
		oldToken.UpdatedAt = now
		m.tokens[tokenID] = oldToken
		return AccessToken{}, AccessToken{}, "", ErrTokenExpired
	}

	replacementSecret := "dope_" + randomHex(24, "fallback")
	replacement := AccessToken{
		TokenID:            "tok_" + randomHex(8, "fallback"),
		PrincipalID:        oldToken.PrincipalID,
		Label:              oldToken.Label,
		Mode:               oldToken.Mode,
		TokenHash:          hashSecret(replacementSecret),
		TokenPreview:       replacementSecret[:12],
		Status:             "active",
		DefaultTenantID:    oldToken.DefaultTenantID,
		CreatedAt:          now,
		UpdatedAt:          now,
		ExpiresAt:          input.ExpiresAt,
		RotatedFromTokenID: oldToken.TokenID,
	}
	oldToken.Status = "rotated"
	oldToken.UpdatedAt = now
	oldToken.RotatedToTokenID = replacement.TokenID
	m.tokens[oldToken.TokenID] = oldToken
	m.tokens[replacement.TokenID] = replacement
	m.tokenIDs = append(m.tokenIDs, replacement.TokenID)
	return oldToken, replacement, replacementSecret, nil
}

func (m *Manager) Authenticate(tokenSecret string) (AccessToken, error) {
	if tokenSecret == "" {
		return AccessToken{}, ErrAuthRequired
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	tokenHash := hashSecret(tokenSecret)
	for tokenID, token := range m.tokens {
		if token.TokenHash != tokenHash {
			continue
		}
		switch token.Status {
		case "", "active":
		case "revoked":
			return token, ErrTokenRevoked
		case "expired":
			return token, ErrTokenExpired
		case "rotated":
			return token, ErrTokenRotated
		default:
			return AccessToken{}, ErrTokenInvalid
		}
		now := time.Now().UTC()
		if token.ExpiresAt != nil && !token.ExpiresAt.After(now) {
			token.Status = "expired"
			token.UpdatedAt = now
			m.tokens[tokenID] = token
			return token, ErrTokenExpired
		}
		token.LastUsedAt = &now
		token.UpdatedAt = now
		m.tokens[tokenID] = token
		return token, nil
	}

	return AccessToken{}, ErrTokenInvalid
}

func (m *Manager) GetToken(tokenID string) (AccessToken, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	token, ok := m.tokens[tokenID]
	return token, ok
}

func (m *Manager) UpdateToken(token AccessToken) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tokens[token.TokenID]; !ok {
		m.tokenIDs = append(m.tokenIDs, token.TokenID)
	}
	m.tokens[token.TokenID] = token
}

func (m *Manager) ListTokens() []AccessToken {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]AccessToken, 0, len(m.tokenIDs))
	for _, tokenID := range m.tokenIDs {
		items = append(items, m.tokens[tokenID])
	}
	return items
}

func (m *Manager) GetPairing(pairingID string) (Pairing, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pairing, ok := m.pairings[pairingID]
	return pairing, ok
}

func (m *Manager) Restore(pairings []Pairing, tokens []AccessToken) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.pairings = make(map[string]Pairing, len(pairings))
	m.pairingIDs = make([]string, 0, len(pairings))
	for _, pairing := range pairings {
		m.pairings[pairing.PairingID] = pairing
		m.pairingIDs = append(m.pairingIDs, pairing.PairingID)
	}

	m.tokens = make(map[string]AccessToken, len(tokens))
	m.tokenIDs = make([]string, 0, len(tokens))
	for _, token := range tokens {
		m.tokens[token.TokenID] = token
		m.tokenIDs = append(m.tokenIDs, token.TokenID)
	}
}

func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func randomDigits(length int) string {
	if length <= 0 {
		return ""
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%0*d", length, 0)
	}
	for i := range buf {
		buf[i] = '0' + (buf[i] % 10)
	}
	return string(buf)
}

func randomHex(size int, fallback string) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return fallback
	}
	return hex.EncodeToString(buf)
}
