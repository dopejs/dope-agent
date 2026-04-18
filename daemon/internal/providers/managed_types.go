package providers

import "time"

type AuthStatus string

const (
	AuthStatusUnknown        AuthStatus = "unknown"
	AuthStatusLoginRequired  AuthStatus = "login_required"
	AuthStatusPendingLogin   AuthStatus = "pending_login"
	AuthStatusAuthenticated  AuthStatus = "authenticated"
	AuthStatusRevoked        AuthStatus = "revoked"
	AuthStatusError          AuthStatus = "error"
)

type AuthState struct {
	ProviderID           string            `json:"providerId"`
	Family               Family            `json:"family"`
	AuthMode             AuthMode          `json:"authMode"`
	Status               AuthStatus        `json:"status"`
	CLIPath              string            `json:"cliPath,omitempty"`
	CLIAvailable         bool              `json:"cliAvailable"`
	AccountLabel         string            `json:"accountLabel,omitempty"`
	AccountID            string            `json:"accountId,omitempty"`
	Plan                 string            `json:"plan,omitempty"`
	AuthMethod           string            `json:"authMethod,omitempty"`
	LoginCommand         []string          `json:"loginCommand,omitempty"`
	LogoutCommand        []string          `json:"logoutCommand,omitempty"`
	LastCheckedAt        time.Time         `json:"lastCheckedAt"`
	LastAuthenticatedAt  *time.Time        `json:"lastAuthenticatedAt,omitempty"`
	LastError            string            `json:"lastError,omitempty"`
	Metadata             map[string]string `json:"metadata,omitempty"`
}

type Model struct {
	ProviderID      string   `json:"providerId"`
	ModelID         string   `json:"modelId"`
	DisplayName     string   `json:"displayName"`
	Description     string   `json:"description,omitempty"`
	Default         bool     `json:"default"`
	Available       bool     `json:"available"`
	Source          string   `json:"source"`
	Chat            bool     `json:"chat"`
	Stream          bool     `json:"stream"`
	Coding          bool     `json:"coding"`
	ToolUse         bool     `json:"toolUse"`
	ReasoningLevels []string `json:"reasoningLevels,omitempty"`
}

type Preference struct {
	ProviderID    string    `json:"providerId"`
	DefaultModel  string    `json:"defaultModel"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
