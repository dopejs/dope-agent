package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/profiles"
)

var ErrProfileNotFound = errors.New("agent profile not found")

func (s *SQLiteStore) EnsureDefaultAgentProfile(ctx context.Context, tenantID string) (profiles.AgentProfile, error) {
	items, err := s.ListAgentProfiles(ctx, tenantID, 1)
	if err != nil {
		return profiles.AgentProfile{}, err
	}
	if len(items.Items) > 0 {
		return items.Items[0], nil
	}
	result, err := s.CreateAgentProfile(ctx, identity.TenantContext{TenantID: tenantID, PrincipalID: "system"}, profiles.MutationInput{
		DisplayName:     "Default Agent",
		DisplayIdentity: profiles.DisplayIdentity{Name: "DopeAgent", SafeSummary: "Default personal assistant profile"},
		Persona:         profiles.Persona{Tone: "direct", SafeSummary: "Concise production-oriented behavior"},
		DefaultProviderPreference: profiles.DefaultProviderPreference{
			ValidationState: profiles.OverlayValid,
		},
		SafetyDefaults: profiles.SafetyDefaults{
			ApprovalPosture: "ask_for_risky_changes",
			ValidationState: profiles.OverlayValid,
		},
		LegacyMappingEvidence: profiles.DefaultLegacyMappingEvidence(),
		Activate:              true,
		ReasonCode:            "default_seeded",
		OverlayReferences:     profiles.DefaultLegacyOverlayReferenceInputs(),
	})
	if err != nil {
		return profiles.AgentProfile{}, err
	}
	return result.Profile, nil
}

func (s *SQLiteStore) ListAgentProfiles(ctx context.Context, tenantID string, limit int) (profiles.ListResponse, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.document_json,
		       CASE WHEN s.profile_id IS NOT NULL THEN 1 ELSE 0 END,
		       (SELECT COUNT(1) FROM agent_profile_overlay_references o WHERE o.tenant_id = p.tenant_id AND o.profile_id = p.profile_id AND o.profile_version_id = p.active_version_id)
		FROM agent_profiles p
		LEFT JOIN agent_profile_active_selections s
		  ON s.tenant_id = p.tenant_id AND s.profile_id = p.profile_id AND s.selection_scope = 'tenant_default'
		WHERE p.tenant_id = ?
		ORDER BY p.updated_at DESC, p.profile_id DESC
		LIMIT ?`, tenantID, limit)
	if err != nil {
		return profiles.ListResponse{}, err
	}
	defer rows.Close()
	items := make([]profiles.AgentProfile, 0)
	for rows.Next() {
		var doc []byte
		var tenantDefault int
		var overlayCount int
		if err := rows.Scan(&doc, &tenantDefault, &overlayCount); err != nil {
			return profiles.ListResponse{}, err
		}
		var profile profiles.AgentProfile
		if err := json.Unmarshal(doc, &profile); err != nil {
			return profiles.ListResponse{}, err
		}
		profile.TenantDefault = tenantDefault == 1
		profile.OverlayReferenceCount = overlayCount
		items = append(items, profiles.RedactProfile(profile))
	}
	return profiles.ListResponse{TenantID: tenantID, Page: profiles.Page{Limit: limit, Order: "updated_at_desc"}, Items: items}, rows.Err()
}

func (s *SQLiteStore) GetAgentProfileDetail(ctx context.Context, tenantID, profileID string) (profiles.ProfileDetail, bool, error) {
	profile, found, err := s.getAgentProfile(ctx, tenantID, profileID)
	if err != nil || !found {
		return profiles.ProfileDetail{}, found, err
	}
	versions, err := s.ListAgentProfileVersions(ctx, tenantID, profileID, 50)
	if err != nil {
		return profiles.ProfileDetail{}, false, err
	}
	overlays, err := s.listAgentProfileOverlays(ctx, tenantID, profileID)
	if err != nil {
		return profiles.ProfileDetail{}, false, err
	}
	audits, err := s.listAgentProfileAuditEvents(ctx, tenantID, profileID, 25)
	if err != nil {
		return profiles.ProfileDetail{}, false, err
	}
	return profiles.ProfileDetail{Profile: profiles.RedactProfile(profile), Versions: versions, OverlayReferences: overlays, AuditEvents: audits}, true, nil
}

func (s *SQLiteStore) CreateAgentProfile(ctx context.Context, actor identity.TenantContext, input profiles.MutationInput) (profiles.MutationResult, error) {
	if strings.TrimSpace(actor.TenantID) == "" || strings.TrimSpace(actor.PrincipalID) == "" {
		return profiles.MutationResult{}, profiles.ErrExplicitActorRequired
	}
	if err := profiles.ValidateMutation(input); err != nil {
		return profiles.MutationResult{}, err
	}
	if err := s.validateProfileMutationAgainstStore(ctx, input); err != nil {
		return profiles.MutationResult{}, err
	}
	now := time.Now().UTC()
	profileID := newStoreID("prof")
	versionID := newStoreID("profv")
	auditID := newStoreID("audit_profile")
	status := profiles.StatusDraft
	if input.Activate {
		status = profiles.StatusActive
	}
	profile := profiles.AgentProfile{
		ProfileID:                 profileID,
		TenantID:                  actor.TenantID,
		DisplayName:               strings.TrimSpace(input.DisplayName),
		DisplayIdentity:           input.DisplayIdentity,
		Persona:                   input.Persona,
		DefaultProviderPreference: input.DefaultProviderPreference,
		SafetyDefaults:            input.SafetyDefaults,
		LegacyMappingEvidence:     input.LegacyMappingEvidence,
		Status:                    status,
		ActiveVersionID:           versionID,
		CreatedAt:                 now,
		UpdatedAt:                 now,
		CreatedByPrincipalID:      actor.PrincipalID,
		UpdatedByPrincipalID:      actor.PrincipalID,
		RedactionStatus:           profiles.RedactionRedacted,
	}
	profile = profiles.RedactProfile(profile)
	version := profiles.ProfileVersion{
		ProfileVersionID:    versionID,
		ProfileID:           profileID,
		TenantID:            actor.TenantID,
		VersionNumber:       1,
		ChangeKind:          profiles.ChangeCreated,
		ChangeSummary:       "Created profile",
		Snapshot:            profile,
		RollbackEligibility: profiles.RollbackEligible,
		ActorPrincipalID:    actor.PrincipalID,
		CreatedAt:           now,
		AuditEventID:        auditID,
		RedactionStatus:     profiles.RedactionRedacted,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return profiles.MutationResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := insertAgentProfileTx(ctx, tx, profile); err != nil {
		return profiles.MutationResult{}, err
	}
	if err := insertAgentProfileVersionTx(ctx, tx, version); err != nil {
		return profiles.MutationResult{}, err
	}
	if err := replaceOverlayReferencesTx(ctx, tx, actor.TenantID, profileID, versionID, input.OverlayReferences, now); err != nil {
		return profiles.MutationResult{}, err
	}
	if input.Activate {
		if _, err := upsertActiveSelectionTx(ctx, tx, actor, profile, versionID, profiles.SelectionDefaultSeeded, auditID, now); err != nil {
			return profiles.MutationResult{}, err
		}
	}
	if err := insertProfileAuditTx(ctx, tx, profiles.AuditEvent{AuditEventID: auditID, TenantID: actor.TenantID, ProfileID: profileID, ProfileVersionID: versionID, ActorPrincipalID: actor.PrincipalID, EventKind: "profile.created", Outcome: "succeeded", PermissionGate: string(identity.PermissionProfilesManage), ReasonCode: defaultReason(input.ReasonCode, "user_created_profile"), SafeSummary: "Profile created", OccurredAt: now, RedactionStatus: profiles.RedactionRedacted}); err != nil {
		return profiles.MutationResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return profiles.MutationResult{}, err
	}
	return profiles.MutationResult{Profile: profile, Version: version, AuditEventID: auditID}, nil
}

func (s *SQLiteStore) UpdateAgentProfile(ctx context.Context, actor identity.TenantContext, profileID string, input profiles.MutationInput) (profiles.MutationResult, error) {
	if strings.TrimSpace(actor.TenantID) == "" || strings.TrimSpace(actor.PrincipalID) == "" {
		return profiles.MutationResult{}, profiles.ErrExplicitActorRequired
	}
	if err := profiles.ValidateMutation(input); err != nil {
		return profiles.MutationResult{}, err
	}
	if err := s.validateProfileMutationAgainstStore(ctx, input); err != nil {
		return profiles.MutationResult{}, err
	}
	current, found, err := s.getAgentProfile(ctx, actor.TenantID, profileID)
	if err != nil {
		return profiles.MutationResult{}, err
	}
	if !found {
		return profiles.MutationResult{}, ErrProfileNotFound
	}
	now := time.Now().UTC()
	versionNumber, err := s.nextAgentProfileVersion(ctx, actor.TenantID, profileID)
	if err != nil {
		return profiles.MutationResult{}, err
	}
	sourceVersionID := current.ActiveVersionID
	versionID := newStoreID("profv")
	auditID := newStoreID("audit_profile")
	current.DisplayName = strings.TrimSpace(input.DisplayName)
	current.DisplayIdentity = input.DisplayIdentity
	current.Persona = input.Persona
	current.DefaultProviderPreference = input.DefaultProviderPreference
	current.SafetyDefaults = input.SafetyDefaults
	if input.LegacyMappingEvidence != nil {
		current.LegacyMappingEvidence = input.LegacyMappingEvidence
	}
	current.ActiveVersionID = versionID
	current.UpdatedAt = now
	current.UpdatedByPrincipalID = actor.PrincipalID
	current = profiles.RedactProfile(current)
	version := profiles.ProfileVersion{ProfileVersionID: versionID, ProfileID: profileID, TenantID: actor.TenantID, VersionNumber: versionNumber, SourceVersionID: sourceVersionID, ChangeKind: profiles.ChangeUpdated, ChangeSummary: "Updated profile", Snapshot: current, RollbackEligibility: profiles.RollbackEligible, ActorPrincipalID: actor.PrincipalID, CreatedAt: now, AuditEventID: auditID, RedactionStatus: profiles.RedactionRedacted}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return profiles.MutationResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := updateAgentProfileTx(ctx, tx, current); err != nil {
		return profiles.MutationResult{}, err
	}
	if err := insertAgentProfileVersionTx(ctx, tx, version); err != nil {
		return profiles.MutationResult{}, err
	}
	if err := replaceOverlayReferencesTx(ctx, tx, actor.TenantID, profileID, versionID, input.OverlayReferences, now); err != nil {
		return profiles.MutationResult{}, err
	}
	if err := insertProfileAuditTx(ctx, tx, profiles.AuditEvent{AuditEventID: auditID, TenantID: actor.TenantID, ProfileID: profileID, ProfileVersionID: versionID, ActorPrincipalID: actor.PrincipalID, EventKind: "profile.updated", Outcome: "succeeded", PermissionGate: string(identity.PermissionProfilesManage), ReasonCode: defaultReason(input.ReasonCode, "user_updated_profile"), SafeSummary: "Profile updated", OccurredAt: now, RedactionStatus: profiles.RedactionRedacted}); err != nil {
		return profiles.MutationResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return profiles.MutationResult{}, err
	}
	return profiles.MutationResult{Profile: current, Version: version, AuditEventID: auditID}, nil
}

func (s *SQLiteStore) ActivateAgentProfile(ctx context.Context, actor identity.TenantContext, profileID string, input profiles.ActivationInput) (profiles.ActiveSelection, error) {
	if strings.TrimSpace(actor.TenantID) == "" || strings.TrimSpace(actor.PrincipalID) == "" {
		return profiles.ActiveSelection{}, profiles.ErrExplicitActorRequired
	}
	profile, found, err := s.getAgentProfile(ctx, actor.TenantID, profileID)
	if err != nil {
		return profiles.ActiveSelection{}, err
	}
	if !found {
		return profiles.ActiveSelection{}, ErrProfileNotFound
	}
	versionID := input.ProfileVersionID
	if versionID == "" {
		versionID = profile.ActiveVersionID
	}
	version, found, err := s.getAgentProfileVersion(ctx, actor.TenantID, profileID, versionID)
	if err != nil {
		return profiles.ActiveSelection{}, err
	}
	if !found {
		return profiles.ActiveSelection{}, ErrProfileNotFound
	}
	if err := profiles.CanActivate(profile, version); err != nil {
		return profiles.ActiveSelection{}, err
	}
	now := time.Now().UTC()
	auditID := newStoreID("audit_profile")
	profile.Status = profiles.StatusActive
	profile.ActiveVersionID = versionID
	profile.UpdatedAt = now
	profile.UpdatedByPrincipalID = actor.PrincipalID
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return profiles.ActiveSelection{}, err
	}
	defer func() { _ = tx.Rollback() }()
	selection, err := upsertActiveSelectionTx(ctx, tx, actor, profile, versionID, profiles.SelectionUserActivated, auditID, now)
	if err != nil {
		return profiles.ActiveSelection{}, err
	}
	if err := updateAgentProfileTx(ctx, tx, profile); err != nil {
		return profiles.ActiveSelection{}, err
	}
	if err := insertProfileAuditTx(ctx, tx, profiles.AuditEvent{AuditEventID: auditID, TenantID: actor.TenantID, ProfileID: profileID, ProfileVersionID: versionID, ActorPrincipalID: actor.PrincipalID, EventKind: "profile.activated", Outcome: "succeeded", PermissionGate: string(identity.PermissionProfilesManage), ReasonCode: defaultReason(input.ReasonCode, "user_selected_default"), SafeSummary: "Profile activated", OccurredAt: now, RedactionStatus: profiles.RedactionRedacted}); err != nil {
		return profiles.ActiveSelection{}, err
	}
	if err := tx.Commit(); err != nil {
		return profiles.ActiveSelection{}, err
	}
	return selection, nil
}

func (s *SQLiteStore) ListAgentProfileVersions(ctx context.Context, tenantID, profileID string, limit int) ([]profiles.ProfileVersion, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT document_json FROM agent_profile_versions WHERE tenant_id = ? AND profile_id = ? ORDER BY version_number DESC LIMIT ?`, tenantID, profileID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []profiles.ProfileVersion
	for rows.Next() {
		var doc []byte
		if err := rows.Scan(&doc); err != nil {
			return nil, err
		}
		var item profiles.ProfileVersion
		if err := json.Unmarshal(doc, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) RollbackAgentProfile(ctx context.Context, actor identity.TenantContext, profileID string, input profiles.RollbackInput) (profiles.MutationResult, error) {
	if strings.TrimSpace(actor.TenantID) == "" || strings.TrimSpace(actor.PrincipalID) == "" {
		return profiles.MutationResult{}, profiles.ErrExplicitActorRequired
	}
	if strings.TrimSpace(input.SourceProfileVersionID) == "" {
		return profiles.MutationResult{}, profiles.ErrProfileNotActivatable
	}
	source, found, err := s.getAgentProfileVersion(ctx, actor.TenantID, profileID, input.SourceProfileVersionID)
	if err != nil {
		return profiles.MutationResult{}, err
	}
	if !found {
		return profiles.MutationResult{}, ErrProfileNotFound
	}
	current, found, err := s.getAgentProfile(ctx, actor.TenantID, profileID)
	if err != nil {
		return profiles.MutationResult{}, err
	}
	if !found {
		return profiles.MutationResult{}, ErrProfileNotFound
	}
	if profiles.RollbackEligibilityFor(current, source) != profiles.RollbackEligible {
		return profiles.MutationResult{}, profiles.ErrProfileNotActivatable
	}
	wasTenantDefault, err := s.isTenantDefaultProfile(ctx, actor.TenantID, profileID)
	if err != nil {
		return profiles.MutationResult{}, err
	}
	sourceOverlays, err := s.listAgentProfileOverlaysForVersion(ctx, actor.TenantID, profileID, source.ProfileVersionID)
	if err != nil {
		return profiles.MutationResult{}, err
	}
	overlayInputs := make([]profiles.OverlayReferenceInput, 0, len(sourceOverlays))
	for _, overlay := range sourceOverlays {
		overlayInputs = append(overlayInputs, profiles.OverlayReferenceInput{
			ReferenceKind: overlay.ReferenceKind,
			ReferenceURI:  overlay.ReferenceURI,
			Scope:         overlay.Scope,
		})
	}
	if err := profiles.ValidateMutation(profiles.MutationInput{
		DisplayName:               source.Snapshot.DisplayName,
		DisplayIdentity:           source.Snapshot.DisplayIdentity,
		Persona:                   source.Snapshot.Persona,
		DefaultProviderPreference: source.Snapshot.DefaultProviderPreference,
		SafetyDefaults:            source.Snapshot.SafetyDefaults,
		LegacyMappingEvidence:     source.Snapshot.LegacyMappingEvidence,
		OverlayReferences:         overlayInputs,
	}); err != nil {
		return profiles.MutationResult{}, err
	}
	if err := s.validateProfileMutationAgainstStore(ctx, profiles.MutationInput{
		DisplayName:               source.Snapshot.DisplayName,
		DisplayIdentity:           source.Snapshot.DisplayIdentity,
		Persona:                   source.Snapshot.Persona,
		DefaultProviderPreference: source.Snapshot.DefaultProviderPreference,
		SafetyDefaults:            source.Snapshot.SafetyDefaults,
		LegacyMappingEvidence:     source.Snapshot.LegacyMappingEvidence,
		OverlayReferences:         overlayInputs,
	}); err != nil {
		return profiles.MutationResult{}, err
	}
	now := time.Now().UTC()
	versionNumber, err := s.nextAgentProfileVersion(ctx, actor.TenantID, profileID)
	if err != nil {
		return profiles.MutationResult{}, err
	}
	versionID := newStoreID("profv")
	auditID := newStoreID("audit_profile")
	current.DisplayName = strings.TrimSpace(source.Snapshot.DisplayName)
	current.DisplayIdentity = source.Snapshot.DisplayIdentity
	current.Persona = source.Snapshot.Persona
	current.DefaultProviderPreference = source.Snapshot.DefaultProviderPreference
	current.SafetyDefaults = source.Snapshot.SafetyDefaults
	current.LegacyMappingEvidence = source.Snapshot.LegacyMappingEvidence
	current.ActiveVersionID = versionID
	current.UpdatedAt = now
	current.UpdatedByPrincipalID = actor.PrincipalID
	current = profiles.RedactProfile(current)
	version := profiles.ProfileVersion{
		ProfileVersionID:    versionID,
		ProfileID:           profileID,
		TenantID:            actor.TenantID,
		VersionNumber:       versionNumber,
		SourceVersionID:     source.ProfileVersionID,
		ChangeKind:          profiles.ChangeRolledBack,
		ChangeSummary:       "Rolled back profile",
		Snapshot:            current,
		RollbackEligibility: profiles.RollbackEligible,
		ActorPrincipalID:    actor.PrincipalID,
		CreatedAt:           now,
		AuditEventID:        auditID,
		RedactionStatus:     profiles.RedactionRedacted,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return profiles.MutationResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := updateAgentProfileTx(ctx, tx, current); err != nil {
		return profiles.MutationResult{}, err
	}
	if err := insertAgentProfileVersionTx(ctx, tx, version); err != nil {
		return profiles.MutationResult{}, err
	}
	if err := replaceOverlayReferencesTx(ctx, tx, actor.TenantID, profileID, versionID, overlayInputs, now); err != nil {
		return profiles.MutationResult{}, err
	}
	var selection profiles.ActiveSelection
	if wasTenantDefault {
		selection, err = upsertActiveSelectionTx(ctx, tx, actor, current, versionID, profiles.SelectionRollbackActivated, auditID, now)
		if err != nil {
			return profiles.MutationResult{}, err
		}
	}
	if err := insertProfileAuditTx(ctx, tx, profiles.AuditEvent{AuditEventID: auditID, TenantID: actor.TenantID, ProfileID: profileID, ProfileVersionID: versionID, ActorPrincipalID: actor.PrincipalID, EventKind: "profile.rolled_back", Outcome: "succeeded", PermissionGate: string(identity.PermissionProfilesManage), ReasonCode: defaultReason(input.ReasonCode, "operator_reverted_persona"), SafeSummary: "Profile rolled back", OccurredAt: now, RedactionStatus: profiles.RedactionRedacted}); err != nil {
		return profiles.MutationResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return profiles.MutationResult{}, err
	}
	return profiles.MutationResult{Profile: current, Version: version, Selection: selection, AuditEventID: auditID}, nil
}

func (s *SQLiteStore) RetireAgentProfile(ctx context.Context, actor identity.TenantContext, profileID string, status profiles.Status, input profiles.RetirementInput) (profiles.MutationResult, error) {
	if strings.TrimSpace(actor.TenantID) == "" || strings.TrimSpace(actor.PrincipalID) == "" {
		return profiles.MutationResult{}, profiles.ErrExplicitActorRequired
	}
	profile, found, err := s.getAgentProfile(ctx, actor.TenantID, profileID)
	if err != nil {
		return profiles.MutationResult{}, err
	}
	if !found {
		return profiles.MutationResult{}, ErrProfileNotFound
	}
	if status != profiles.StatusArchived && status != profiles.StatusDisabled {
		return profiles.MutationResult{}, fmt.Errorf("unsupported retirement status %s", status)
	}
	wasTenantDefault, err := s.isTenantDefaultProfile(ctx, actor.TenantID, profileID)
	if err != nil {
		return profiles.MutationResult{}, err
	}
	now := time.Now().UTC()
	versionNumber, err := s.nextAgentProfileVersion(ctx, actor.TenantID, profileID)
	if err != nil {
		return profiles.MutationResult{}, err
	}
	versionID := newStoreID("profv")
	auditID := newStoreID("audit_profile")
	profile.Status = status
	profile.ActiveVersionID = versionID
	profile.UpdatedAt = now
	profile.UpdatedByPrincipalID = actor.PrincipalID
	if status == profiles.StatusArchived {
		profile.ArchivedAt = &now
	} else {
		profile.DisabledAt = &now
	}
	version := profiles.ProfileVersion{ProfileVersionID: versionID, ProfileID: profileID, TenantID: actor.TenantID, VersionNumber: versionNumber, ChangeKind: profiles.ChangeArchived, ChangeSummary: "Retired profile", Snapshot: profile, RollbackEligibility: profiles.RollbackProfileArchived, ActorPrincipalID: actor.PrincipalID, CreatedAt: now, AuditEventID: auditID, RedactionStatus: profiles.RedactionRedacted}
	eventKind := "profile.archived"
	if status == profiles.StatusDisabled {
		version.ChangeKind = profiles.ChangeDisabled
		version.RollbackEligibility = profiles.RollbackProfileDisabled
		eventKind = "profile.disabled"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return profiles.MutationResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := updateAgentProfileTx(ctx, tx, profile); err != nil {
		return profiles.MutationResult{}, err
	}
	if err := insertAgentProfileVersionTx(ctx, tx, version); err != nil {
		return profiles.MutationResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM agent_profile_active_selections WHERE tenant_id = ? AND profile_id = ?`, actor.TenantID, profileID); err != nil {
		return profiles.MutationResult{}, err
	}
	var selection profiles.ActiveSelection
	if wasTenantDefault {
		defaultProfile, ensureErr := ensureDefaultAgentProfileTx(ctx, tx, actor.TenantID, now)
		if ensureErr != nil {
			return profiles.MutationResult{}, ensureErr
		}
		selection, err = upsertActiveSelectionTx(ctx, tx, identity.TenantContext{TenantID: actor.TenantID, PrincipalID: "system"}, defaultProfile, defaultProfile.ActiveVersionID, profiles.SelectionSystemFallback, auditID, now)
		if err != nil {
			return profiles.MutationResult{}, err
		}
	}
	if err := insertProfileAuditTx(ctx, tx, profiles.AuditEvent{AuditEventID: auditID, TenantID: actor.TenantID, ProfileID: profileID, ProfileVersionID: versionID, ActorPrincipalID: actor.PrincipalID, EventKind: eventKind, Outcome: "succeeded", PermissionGate: string(identity.PermissionProfilesManage), ReasonCode: defaultReason(input.ReasonCode, "operator_retired_profile"), SafeSummary: "Profile retired", OccurredAt: now, RedactionStatus: profiles.RedactionRedacted}); err != nil {
		return profiles.MutationResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return profiles.MutationResult{}, err
	}
	return profiles.MutationResult{Profile: profile, Version: version, Selection: selection, AuditEventID: auditID}, nil
}

func (s *SQLiteStore) getAgentProfile(ctx context.Context, tenantID, profileID string) (profiles.AgentProfile, bool, error) {
	var doc []byte
	err := s.db.QueryRowContext(ctx, `SELECT document_json FROM agent_profiles WHERE tenant_id = ? AND profile_id = ?`, tenantID, profileID).Scan(&doc)
	if errors.Is(err, sql.ErrNoRows) {
		return profiles.AgentProfile{}, false, nil
	}
	if err != nil {
		return profiles.AgentProfile{}, false, err
	}
	var profile profiles.AgentProfile
	if err := json.Unmarshal(doc, &profile); err != nil {
		return profiles.AgentProfile{}, false, err
	}
	return profile, true, nil
}

func (s *SQLiteStore) validateProfileMutationAgainstStore(ctx context.Context, input profiles.MutationInput) error {
	providerID := strings.TrimSpace(input.DefaultProviderPreference.ProviderID)
	if providerID == "" {
		return nil
	}
	models, err := s.ListProviderModels(ctx)
	if err != nil {
		return err
	}
	modelID := strings.TrimSpace(input.DefaultProviderPreference.Model)
	providerKnown := false
	availableDefault := false
	availableAny := false
	for _, model := range models {
		if strings.TrimSpace(model.ProviderID) != providerID {
			continue
		}
		providerKnown = true
		if model.Available {
			availableAny = true
		}
		if modelID == "" {
			if model.Default && model.Available {
				availableDefault = true
			}
			continue
		}
		if strings.TrimSpace(model.ModelID) != modelID {
			continue
		}
		if !model.Available {
			return profiles.InvalidProfileReason("provider_model_unavailable")
		}
		if input.DefaultProviderPreference.ReasoningLevel != "" && !stringInSlice(input.DefaultProviderPreference.ReasoningLevel, model.ReasoningLevels) {
			return profiles.InvalidProfileReason("reasoning_level_unsupported_for_model")
		}
		return nil
	}
	if !providerKnown {
		return profiles.InvalidProfileReason("provider_not_available")
	}
	if modelID == "" && (availableDefault || availableAny) {
		return nil
	}
	return profiles.InvalidProfileReason("provider_model_not_available")
}

func stringInSlice(value string, items []string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) == strings.TrimSpace(value) {
			return true
		}
	}
	return false
}

func (s *SQLiteStore) isTenantDefaultProfile(ctx context.Context, tenantID, profileID string) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM agent_profile_active_selections WHERE tenant_id = ? AND profile_id = ? AND selection_scope = ?`, tenantID, profileID, profiles.SelectionScopeTenantDefault).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *SQLiteStore) nextAgentProfileVersion(ctx context.Context, tenantID, profileID string) (int, error) {
	var maxVersion sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(version_number) FROM agent_profile_versions WHERE tenant_id = ? AND profile_id = ?`, tenantID, profileID).Scan(&maxVersion); err != nil {
		return 0, err
	}
	if !maxVersion.Valid {
		return 1, nil
	}
	return int(maxVersion.Int64) + 1, nil
}

func (s *SQLiteStore) getAgentProfileVersion(ctx context.Context, tenantID, profileID, versionID string) (profiles.ProfileVersion, bool, error) {
	var doc []byte
	err := s.db.QueryRowContext(ctx, `SELECT document_json FROM agent_profile_versions WHERE tenant_id = ? AND profile_id = ? AND profile_version_id = ?`, tenantID, profileID, versionID).Scan(&doc)
	if errors.Is(err, sql.ErrNoRows) {
		return profiles.ProfileVersion{}, false, nil
	}
	if err != nil {
		return profiles.ProfileVersion{}, false, err
	}
	var version profiles.ProfileVersion
	if err := json.Unmarshal(doc, &version); err != nil {
		return profiles.ProfileVersion{}, false, err
	}
	return version, true, nil
}

func (s *SQLiteStore) rewriteProfileVersionDocument(ctx context.Context, version profiles.ProfileVersion) error {
	doc, err := json.Marshal(version)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE agent_profile_versions SET document_json = ? WHERE tenant_id = ? AND profile_id = ? AND profile_version_id = ?`, doc, version.TenantID, version.ProfileID, version.ProfileVersionID)
	return err
}

func insertAgentProfileTx(ctx context.Context, tx *sql.Tx, profile profiles.AgentProfile) error {
	doc, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO agent_profiles (profile_id, tenant_id, display_name, status, active_version_id, created_at, updated_at, archived_at, disabled_at, created_by_principal_id, updated_by_principal_id, redaction_status, document_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		profile.ProfileID, profile.TenantID, profile.DisplayName, profile.Status, profile.ActiveVersionID, profile.CreatedAt.Format(time.RFC3339Nano), profile.UpdatedAt.Format(time.RFC3339Nano), nullableProfileTime(profile.ArchivedAt), nullableProfileTime(profile.DisabledAt), profile.CreatedByPrincipalID, profile.UpdatedByPrincipalID, profile.RedactionStatus, doc)
	return err
}

func ensureDefaultAgentProfileTx(ctx context.Context, tx *sql.Tx, tenantID string, now time.Time) (profiles.AgentProfile, error) {
	var doc []byte
	err := tx.QueryRowContext(ctx, `SELECT document_json FROM agent_profiles WHERE tenant_id = ? AND display_name = ? AND status = ? ORDER BY created_at ASC LIMIT 1`, tenantID, "Default Agent", profiles.StatusActive).Scan(&doc)
	if err == nil {
		var profile profiles.AgentProfile
		if err := json.Unmarshal(doc, &profile); err != nil {
			return profiles.AgentProfile{}, err
		}
		return profile, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return profiles.AgentProfile{}, err
	}
	profileID := newStoreID("prof")
	versionID := newStoreID("profv")
	profile := profiles.AgentProfile{
		ProfileID:                 profileID,
		TenantID:                  tenantID,
		DisplayName:               "Default Agent",
		DisplayIdentity:           profiles.DisplayIdentity{Name: "DopeAgent", SafeSummary: "Default personal assistant profile"},
		Persona:                   profiles.Persona{Tone: "direct", SafeSummary: "Concise production-oriented behavior"},
		DefaultProviderPreference: profiles.DefaultProviderPreference{ValidationState: profiles.OverlayValid},
		SafetyDefaults:            profiles.SafetyDefaults{ApprovalPosture: "ask_for_risky_changes", ValidationState: profiles.OverlayValid},
		LegacyMappingEvidence:     profiles.DefaultLegacyMappingEvidence(),
		Status:                    profiles.StatusActive,
		ActiveVersionID:           versionID,
		CreatedAt:                 now,
		UpdatedAt:                 now,
		CreatedByPrincipalID:      "system",
		UpdatedByPrincipalID:      "system",
		RedactionStatus:           profiles.RedactionRedacted,
	}
	version := profiles.ProfileVersion{
		ProfileVersionID:    versionID,
		ProfileID:           profileID,
		TenantID:            tenantID,
		VersionNumber:       1,
		ChangeKind:          profiles.ChangeCreated,
		ChangeSummary:       "Seeded default profile",
		Snapshot:            profile,
		RollbackEligibility: profiles.RollbackEligible,
		ActorPrincipalID:    "system",
		CreatedAt:           now,
		RedactionStatus:     profiles.RedactionRedacted,
	}
	if err := insertAgentProfileTx(ctx, tx, profile); err != nil {
		return profiles.AgentProfile{}, err
	}
	if err := insertAgentProfileVersionTx(ctx, tx, version); err != nil {
		return profiles.AgentProfile{}, err
	}
	if err := replaceOverlayReferencesTx(ctx, tx, tenantID, profileID, versionID, profiles.DefaultLegacyOverlayReferenceInputs(), now); err != nil {
		return profiles.AgentProfile{}, err
	}
	return profile, nil
}

func updateAgentProfileTx(ctx context.Context, tx *sql.Tx, profile profiles.AgentProfile) error {
	doc, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE agent_profiles SET display_name = ?, status = ?, active_version_id = ?, updated_at = ?, archived_at = ?, disabled_at = ?, updated_by_principal_id = ?, redaction_status = ?, document_json = ? WHERE tenant_id = ? AND profile_id = ?`,
		profile.DisplayName, profile.Status, profile.ActiveVersionID, profile.UpdatedAt.Format(time.RFC3339Nano), nullableProfileTime(profile.ArchivedAt), nullableProfileTime(profile.DisabledAt), profile.UpdatedByPrincipalID, profile.RedactionStatus, doc, profile.TenantID, profile.ProfileID)
	return err
}

func insertAgentProfileVersionTx(ctx context.Context, tx *sql.Tx, version profiles.ProfileVersion) error {
	doc, err := json.Marshal(version)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO agent_profile_versions (profile_version_id, profile_id, tenant_id, version_number, source_version_id, change_kind, change_summary, rollback_eligibility, actor_principal_id, created_at, audit_event_id, redaction_status, document_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		version.ProfileVersionID, version.ProfileID, version.TenantID, version.VersionNumber, version.SourceVersionID, version.ChangeKind, version.ChangeSummary, version.RollbackEligibility, version.ActorPrincipalID, version.CreatedAt.Format(time.RFC3339Nano), version.AuditEventID, version.RedactionStatus, doc)
	return err
}

func upsertActiveSelectionTx(ctx context.Context, tx *sql.Tx, actor identity.TenantContext, profile profiles.AgentProfile, versionID string, reason profiles.SelectionReason, auditID string, now time.Time) (profiles.ActiveSelection, error) {
	selection := profiles.ActiveSelection{SelectionID: newStoreID("sel"), TenantID: actor.TenantID, ProfileID: profile.ProfileID, ProfileVersionID: versionID, SelectionScope: profiles.SelectionScopeTenantDefault, SelectionReason: reason, SelectedByPrincipalID: actor.PrincipalID, SelectedAt: now, AuditEventID: auditID, RedactionStatus: profiles.RedactionRedacted}
	doc, err := json.Marshal(selection)
	if err != nil {
		return profiles.ActiveSelection{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO agent_profile_active_selections (selection_id, tenant_id, profile_id, profile_version_id, selection_scope, selection_reason, selected_by_principal_id, selected_at, audit_event_id, redaction_status, document_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(tenant_id, selection_scope) DO UPDATE SET selection_id = excluded.selection_id, profile_id = excluded.profile_id, profile_version_id = excluded.profile_version_id, selection_reason = excluded.selection_reason, selected_by_principal_id = excluded.selected_by_principal_id, selected_at = excluded.selected_at, audit_event_id = excluded.audit_event_id, redaction_status = excluded.redaction_status, document_json = excluded.document_json`,
		selection.SelectionID, selection.TenantID, selection.ProfileID, selection.ProfileVersionID, selection.SelectionScope, selection.SelectionReason, selection.SelectedByPrincipalID, selection.SelectedAt.Format(time.RFC3339Nano), selection.AuditEventID, selection.RedactionStatus, doc)
	return selection, err
}

func replaceOverlayReferencesTx(ctx context.Context, tx *sql.Tx, tenantID, profileID, versionID string, inputs []profiles.OverlayReferenceInput, now time.Time) error {
	for _, input := range inputs {
		overlay, err := profiles.NormalizeOverlay(input)
		if err != nil {
			return err
		}
		overlay.OverlayReferenceID = newStoreID("ovr")
		overlay.TenantID = tenantID
		overlay.ProfileID = profileID
		overlay.ProfileVersionID = versionID
		overlay.CreatedAt = now
		overlay.UpdatedAt = now
		doc, err := json.Marshal(overlay)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO agent_profile_overlay_references (overlay_reference_id, profile_id, profile_version_id, tenant_id, reference_kind, scope, reference_uri, safe_display_label, validation_state, failure_reason_code, last_validated_at, created_at, updated_at, redaction_status, document_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			overlay.OverlayReferenceID, profileID, versionID, tenantID, overlay.ReferenceKind, overlay.Scope, overlay.ReferenceURI, overlay.SafeDisplayLabel, overlay.ValidationState, overlay.FailureReasonCode, nullableProfileTime(overlay.LastValidatedAt), overlay.CreatedAt.Format(time.RFC3339Nano), overlay.UpdatedAt.Format(time.RFC3339Nano), overlay.RedactionStatus, doc)
		if err != nil {
			return err
		}
	}
	return nil
}

func insertProfileAuditTx(ctx context.Context, tx *sql.Tx, audit profiles.AuditEvent) error {
	doc, err := json.Marshal(audit)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO agent_profile_audit_events (audit_event_id, tenant_id, profile_id, profile_version_id, actor_principal_id, event_kind, outcome, permission_gate, reason_code, safe_summary, occurred_at, redaction_status, document_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		audit.AuditEventID, audit.TenantID, audit.ProfileID, audit.ProfileVersionID, audit.ActorPrincipalID, audit.EventKind, audit.Outcome, audit.PermissionGate, audit.ReasonCode, audit.SafeSummary, audit.OccurredAt.Format(time.RFC3339Nano), audit.RedactionStatus, doc)
	return err
}

func (s *SQLiteStore) listAgentProfileOverlays(ctx context.Context, tenantID, profileID string) ([]profiles.OverlayReference, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT o.document_json
		FROM agent_profile_overlay_references o
		JOIN agent_profiles p ON p.tenant_id = o.tenant_id AND p.profile_id = o.profile_id AND p.active_version_id = o.profile_version_id
		WHERE o.tenant_id = ? AND o.profile_id = ?
		ORDER BY o.created_at ASC`, tenantID, profileID)
	if err != nil {
		return nil, err
	}
	return scanAgentProfileOverlays(rows)
}

func (s *SQLiteStore) listAgentProfileOverlaysForVersion(ctx context.Context, tenantID, profileID, versionID string) ([]profiles.OverlayReference, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT document_json FROM agent_profile_overlay_references WHERE tenant_id = ? AND profile_id = ? AND profile_version_id = ? ORDER BY created_at ASC`, tenantID, profileID, versionID)
	if err != nil {
		return nil, err
	}
	return scanAgentProfileOverlays(rows)
}

func scanAgentProfileOverlays(rows *sql.Rows) ([]profiles.OverlayReference, error) {
	defer rows.Close()
	var items []profiles.OverlayReference
	for rows.Next() {
		var doc []byte
		if err := rows.Scan(&doc); err != nil {
			return nil, err
		}
		var item profiles.OverlayReference
		if err := json.Unmarshal(doc, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) listAgentProfileAuditEvents(ctx context.Context, tenantID, profileID string, limit int) ([]profiles.AuditEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT document_json FROM agent_profile_audit_events WHERE tenant_id = ? AND profile_id = ? ORDER BY occurred_at DESC LIMIT ?`, tenantID, profileID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []profiles.AuditEvent
	for rows.Next() {
		var doc []byte
		if err := rows.Scan(&doc); err != nil {
			return nil, err
		}
		var item profiles.AuditEvent
		if err := json.Unmarshal(doc, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func nullableProfileTime(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func defaultReason(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
