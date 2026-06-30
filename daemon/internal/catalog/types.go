// Package catalog implements the operator-managed skill and capability catalog (Roadmap 68):
// operator-curated catalog items (skills, MCP servers, supervised capabilities) that can be
// enabled, disabled, permissioned, versioned, inspected, and rolled back per tenant. The agent
// does NOT generate or promote its own skills here; hosted install policy is explicit and fails
// closed (unmet requirements or denied permissions block enablement before execution).
package catalog

import "time"

// ItemKind is the kind of catalog item.
type ItemKind string

const (
	ItemKindSkill      ItemKind = "skill"
	ItemKindMCPServer  ItemKind = "mcp_server"
	ItemKindCapability ItemKind = "capability"
)

// TrustTier is the operator-assigned trust level for a catalog item.
type TrustTier string

const (
	TrustTierOfficial  TrustTier = "official"
	TrustTierVerified  TrustTier = "verified"
	TrustTierCommunity TrustTier = "community"
	TrustTierUntrusted TrustTier = "untrusted"
)

// Requirement is a declared prerequisite for a catalog item version (e.g. a sandbox backend, a
// resolved secret, or network access). It is checked before enable/execution (FR-003).
type Requirement struct {
	Key         string `json:"key"`
	Description string `json:"description,omitempty"`
}

// Version is one published version of a catalog item with its source, requirements, and checksum.
type Version struct {
	Version      string        `json:"version"`
	Source       string        `json:"source"`
	Checksum     string        `json:"checksum,omitempty"`
	Requirements []Requirement `json:"requirements,omitempty"`
	PublishedAt  time.Time     `json:"publishedAt"`
}

// CatalogItem is an operator-curated catalog entry. Versions are ordered oldest-first.
type CatalogItem struct {
	ItemID      string    `json:"itemId"`
	Kind        ItemKind  `json:"kind"`
	Name        string    `json:"name"`
	TrustTier   TrustTier `json:"trustTier"`
	Permissions []string  `json:"permissions,omitempty"`
	Versions    []Version `json:"versions"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (c CatalogItem) version(v string) (Version, bool) {
	for _, ver := range c.Versions {
		if ver.Version == v {
			return ver, true
		}
	}
	return Version{}, false
}

func (c CatalogItem) latest() (Version, bool) {
	if len(c.Versions) == 0 {
		return Version{}, false
	}
	return c.Versions[len(c.Versions)-1], true
}

// EnablementState is the per-tenant enablement state of a catalog item.
type EnablementState string

const (
	EnablementEnabled  EnablementState = "enabled"
	EnablementDisabled EnablementState = "disabled"
)

// EnablementEvent is one auditable enablement transition.
type EnablementEvent struct {
	Action     string    `json:"action"` // enabled | disabled | rolled_back
	Version    string    `json:"version,omitempty"`
	Actor      string    `json:"actor,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	OccurredAt time.Time `json:"occurredAt"`
}

// Enablement is the per-tenant enablement record for a catalog item, including the active version
// and an audit history of transitions.
type Enablement struct {
	TenantID      string            `json:"tenantId"`
	ItemID        string            `json:"itemId"`
	State         EnablementState   `json:"state"`
	ActiveVersion string            `json:"activeVersion,omitempty"`
	VersionStack  []string          `json:"versionStack,omitempty"` // enabled-version stack for deterministic rollback
	History       []EnablementEvent `json:"history"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

// Inspection projects a catalog item plus a tenant's enablement and any unmet requirements for
// the active/target version, so a user can see why a skill is unavailable (FR-005, US3).
type Inspection struct {
	Item                CatalogItem   `json:"item"`
	Enablement          Enablement    `json:"enablement"`
	UnmetRequirements   []Requirement `json:"unmetRequirements,omitempty"`
	PermissionSatisfied bool          `json:"permissionSatisfied"`
}
