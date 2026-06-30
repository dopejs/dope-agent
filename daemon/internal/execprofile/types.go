// Package execprofile projects execution backend + sandbox profile availability, requirements,
// health, risk, and selection for product surfaces (Roadmap 69). The daemon policy/sandbox layer
// remains authoritative for execution permission — these projections never grant hidden access
// and never weaken preflight/approval gates; hosted defaults fail closed when a backend is
// unavailable.
package execprofile

import "time"

// BackendKind is the execution backend a profile runs on.
type BackendKind string

const (
	BackendSubprocess BackendKind = "subprocess"
	BackendDocker     BackendKind = "docker"
	BackendSSH        BackendKind = "ssh"
	BackendLocalShell BackendKind = "local_shell"
)

// RiskTier classifies a profile's risk.
type RiskTier string

const (
	RiskLow    RiskTier = "low"
	RiskMedium RiskTier = "medium"
	RiskHigh   RiskTier = "high"
)

// HealthStatus is the live readiness of a profile's backend.
type HealthStatus string

const (
	HealthReady       HealthStatus = "ready"
	HealthDegraded    HealthStatus = "degraded"
	HealthUnavailable HealthStatus = "unavailable"
)

// ExecutionProfile describes an execution backend + sandbox profile: what capabilities it
// provides to tools, what environment prerequisites it needs to be available, and its risk tier.
type ExecutionProfile struct {
	ProfileID    string      `json:"profileId"`
	Name         string      `json:"name"`
	BackendKind  BackendKind `json:"backendKind"`
	RiskTier     RiskTier    `json:"riskTier"`
	Provides     []string    `json:"provides,omitempty"`     // capabilities offered to tools (e.g. docker, network, local_fs)
	Requirements []string    `json:"requirements,omitempty"` // env prerequisites for the profile itself
	Description  string      `json:"description,omitempty"`
	CreatedAt    time.Time   `json:"createdAt"`
}

// ProfileStatus is the live status of a profile: backend health plus any unmet requirements.
type ProfileStatus struct {
	ProfileID         string       `json:"profileId"`
	Health            HealthStatus `json:"health"`
	Reason            string       `json:"reason,omitempty"`
	UnmetRequirements []string     `json:"unmetRequirements,omitempty"`
	Available         bool         `json:"available"` // health ready AND requirements met
}

// ProfileProjection is a profile with its live status (the list/detail product projection).
type ProfileProjection struct {
	Profile ExecutionProfile `json:"profile"`
	Status  ProfileStatus    `json:"status"`
}

// DenialExplanation explains why a tool with the given required capabilities can or cannot run:
// the eligible profiles, plus per-profile missing capabilities (incompatible) or unavailability
// reasons (FR-002 denials link to missing requirements / policy).
type DenialExplanation struct {
	RequiredCapabilities []string            `json:"requiredCapabilities"`
	EligibleProfiles     []string            `json:"eligibleProfiles"`
	MissingCapabilities  map[string][]string `json:"missingCapabilities,omitempty"` // profileId -> missing caps
	Unavailable          map[string]string   `json:"unavailable,omitempty"`         // profileId -> reason
}

// Compatibility reports the profiles compatible/incompatible with a catalog item's capability
// requirements (FR-003).
type Compatibility struct {
	RequiredCapabilities []string `json:"requiredCapabilities"`
	Compatible           []string `json:"compatible"`
	Incompatible         []string `json:"incompatible"`
}

// SelectionEvent is one auditable profile-selection transition.
type SelectionEvent struct {
	ProfileID  string    `json:"profileId"`
	Actor      string    `json:"actor,omitempty"`
	OccurredAt time.Time `json:"occurredAt"`
}

// Selection is a tenant's selected execution profile with an audit history.
type Selection struct {
	TenantID  string           `json:"tenantId"`
	ProfileID string           `json:"profileId"`
	History   []SelectionEvent `json:"history"`
	UpdatedAt time.Time        `json:"updatedAt"`
}
