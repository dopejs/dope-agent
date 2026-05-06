package activation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/billing"
	"github.com/dopejs/dope-agent/daemon/internal/identity"
)

type StateStore interface {
	UpsertActivationState(ctx context.Context, state State) error
	GetActivationState(ctx context.Context, activationID string) (State, bool, error)
	GetActivationStateForPrincipalTenant(ctx context.Context, principalID, tenantID string) (State, bool, error)
}

type IdentityRepository interface {
	GetPrincipal(ctx context.Context, principalID string) (identity.Principal, bool, error)
	ListPrincipals(ctx context.Context, filter identity.PrincipalFilter) ([]identity.Principal, error)
	UpsertPrincipal(ctx context.Context, principal identity.Principal) error
	GetTenant(ctx context.Context, tenantID string) (identity.Tenant, bool, error)
	ListTenants(ctx context.Context, filter identity.TenantFilter) ([]identity.Tenant, error)
	UpsertTenant(ctx context.Context, tenant identity.Tenant) error
	ListMemberships(ctx context.Context, filter identity.MembershipFilter) ([]identity.Membership, error)
	UpsertMembership(ctx context.Context, membership identity.Membership) error
	ListTokenTenantGrants(ctx context.Context, tokenID string) ([]identity.TokenTenantGrant, error)
	UpsertTokenTenantGrant(ctx context.Context, grant identity.TokenTenantGrant) error
}

type BillingProjector interface {
	UsageSummary(ctx context.Context, tenantID string, hosted bool) (billing.UsageSummary, error)
}

type ChatRunner interface {
	RunActivationTestChat(ctx context.Context, input TestChatInput) (TestChatResult, error)
}

type AuditSink interface {
	AppendTenantAuditEvent(ctx context.Context, event identity.TenantAuditEvent) (identity.TenantAuditEvent, error)
}

type Dependencies struct {
	StateStore       StateStore
	Identity         IdentityRepository
	Billing          BillingProjector
	Chat             ChatRunner
	Audit            AuditSink
	Now              func() time.Time
	EnvironmentScope string
	Hosted           bool
}

type Service struct {
	stateStore       StateStore
	identity         IdentityRepository
	billing          BillingProjector
	chat             ChatRunner
	audit            AuditSink
	nowFunc          func() time.Time
	environmentScope string
	hosted           bool
}

func NewService(deps Dependencies) *Service {
	nowFunc := deps.Now
	if nowFunc == nil {
		nowFunc = func() time.Time { return time.Now().UTC() }
	}
	environmentScope := deps.EnvironmentScope
	if environmentScope == "" {
		environmentScope = "test"
	}
	return &Service{
		stateStore:       deps.StateStore,
		identity:         deps.Identity,
		billing:          deps.Billing,
		chat:             deps.Chat,
		audit:            deps.Audit,
		nowFunc:          nowFunc,
		environmentScope: environmentScope,
		hosted:           deps.Hosted,
	}
}

func (s *Service) now() time.Time {
	if s == nil || s.nowFunc == nil {
		return time.Now().UTC()
	}
	return s.nowFunc().UTC()
}

type TestChatInput struct {
	ActivationID     string
	PrincipalID      string
	TenantID         string
	EnvironmentScope string
	Message          string
}

type TestChatResult struct {
	DispatchID   string
	Status       TestChatStatus
	Provider     string
	Model        string
	Usage        map[string]any
	FinishReason string
	CompletedAt  time.Time
}

type ActivateInput struct {
	Token         identity.TokenAuthority
	TenantContext identity.TenantContext
	Source        string
}

type GetInput struct {
	Token         identity.TokenAuthority
	TenantContext identity.TenantContext
}

type RunTestChatInput struct {
	Token         identity.TokenAuthority
	TenantContext identity.TenantContext
	Message       string
}

type Error struct {
	ReasonCode       ReasonCode
	Stage            FailureStage
	Retryable        bool
	RemediationOwner RemediationOwner
	Message          string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return string(e.ReasonCode)
}

func ReasonCodeFromError(err error) ReasonCode {
	var activationErr *Error
	if errors.As(err, &activationErr) {
		return activationErr.ReasonCode
	}
	return ""
}

func (s *Service) Activate(ctx context.Context, input ActivateInput) (State, error) {
	if s == nil || s.identity == nil || s.stateStore == nil {
		return State{}, activationError(ReasonUnexpectedFailed, FailureStageUnexpected, false, RemediationOwnerOperator, "activation service is not configured")
	}
	if input.Token.Status != "" && input.Token.Status != identity.StatusActive {
		return State{}, activationError(ReasonPrincipalDenied, FailureStageEligibility, false, RemediationOwnerProductUser, "activation token is denied")
	}
	principalID := strings.TrimSpace(input.Token.PrincipalID)
	if principalID == "" {
		principalID = strings.TrimSpace(input.TenantContext.PrincipalID)
	}
	if principalID == "" {
		return State{}, activationError(ReasonPrincipalDenied, FailureStageEligibility, false, RemediationOwnerProductUser, "activation principal is required")
	}
	principal, ok, err := s.identity.GetPrincipal(ctx, principalID)
	if err != nil {
		return State{}, err
	}
	now := s.now()
	if !ok {
		principal = identity.Principal{
			PrincipalID:   principalID,
			PrincipalKind: identity.PrincipalKindUser,
			DisplayName:   "Hosted user",
			Status:        identity.StatusActive,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
	}
	if principal.Status == identity.StatusDisabled {
		if err := s.recordAudit(ctx, auditRecord{
			EventKind:   "tenant.activation_denied",
			PrincipalID: principal.PrincipalID,
			TokenID:     input.Token.TokenID,
			Outcome:     identity.AuditOutcomeDenied,
			ReasonCode:  ReasonPrincipalDisabled,
			Stage:       FailureStageEligibility,
			ToStatus:    StatusBlocked,
		}); err != nil {
			return State{}, err
		}
		return State{}, activationError(ReasonPrincipalDisabled, FailureStageEligibility, false, RemediationOwnerProductUser, "activation principal is disabled")
	}
	if principal.Status != "" && principal.Status != identity.StatusActive {
		return State{}, activationError(ReasonPrincipalDenied, FailureStageEligibility, false, RemediationOwnerProductUser, "activation principal is denied")
	}

	tenant, err := s.resolvePersonalTenant(ctx, principal, input.Token, now)
	if err != nil {
		return State{}, err
	}
	if principal.DefaultTenantID != tenant.TenantID {
		principal.DefaultTenantID = tenant.TenantID
		principal.UpdatedAt = now
		if err := s.identity.UpsertPrincipal(ctx, principal); err != nil {
			return State{}, err
		}
	}

	existing, found, err := s.stateStore.GetActivationStateForPrincipalTenant(ctx, principal.PrincipalID, tenant.TenantID)
	if err != nil {
		return State{}, err
	}
	state, err := s.activeStateForPersonalTenant(ctx, principal, tenant, now)
	if err != nil {
		return State{}, err
	}
	if found {
		state.ActivationID = existing.ActivationID
		state.CreatedAt = existing.CreatedAt
		state.TestChat = existing.TestChat
		state.FirstActionCompletedAt = existing.FirstActionCompletedAt
		state.LastTransitionAuditEvent = existing.LastTransitionAuditEvent
		if existing.Status == StatusFirstActionCompleted && state.Status != StatusBlocked {
			state.Status = existing.Status
			state.CurrentStepID = existing.CurrentStepID
			state.CompletedStepIDs = existing.CompletedStepIDs
		}
	}
	if err := s.recordAudit(ctx, auditRecord{
		EventKind:    "tenant.activation_started",
		ActivationID: state.ActivationID,
		TenantID:     tenant.TenantID,
		PrincipalID:  principal.PrincipalID,
		TokenID:      input.Token.TokenID,
		Outcome:      identity.AuditOutcomeSucceeded,
		Stage:        FailureStageTenantResolution,
		FromStatus:   existing.Status,
		ToStatus:     state.Status,
	}); err != nil {
		return State{}, err
	}
	if err := s.stateStore.UpsertActivationState(ctx, state); err != nil {
		return State{}, err
	}
	completionAudit := auditRecord{
		EventKind:    "tenant.activation_completed",
		ActivationID: state.ActivationID,
		TenantID:     tenant.TenantID,
		PrincipalID:  principal.PrincipalID,
		TokenID:      input.Token.TokenID,
		Outcome:      identity.AuditOutcomeSucceeded,
		Stage:        FailureStageTenantResolution,
		FromStatus:   existing.Status,
		ToStatus:     state.Status,
	}
	if state.Status == StatusBlocked && state.FailureReason != nil {
		completionAudit.EventKind = "tenant.activation_blocked"
		completionAudit.Outcome = identity.AuditOutcomeFailedClosed
		completionAudit.Stage = state.FailureReason.Stage
		completionAudit.ReasonCode = state.FailureReason.ReasonCode
		completionAudit.Retryable = state.FailureReason.Retryable
		completionAudit.RemediationOwner = state.FailureReason.RemediationOwner
	}
	if err := s.recordAudit(ctx, completionAudit); err != nil {
		return State{}, err
	}
	return state, nil
}

func (s *Service) Get(ctx context.Context, input GetInput) (State, error) {
	if s == nil || s.stateStore == nil {
		return State{}, activationError(ReasonUnexpectedFailed, FailureStageUnexpected, false, RemediationOwnerOperator, "activation service is not configured")
	}
	principalID := strings.TrimSpace(input.TenantContext.PrincipalID)
	if principalID == "" {
		principalID = strings.TrimSpace(input.Token.PrincipalID)
	}
	tenantID := strings.TrimSpace(input.TenantContext.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(input.Token.DefaultTenantID)
	}
	if principalID == "" || tenantID == "" {
		return State{}, activationError(ReasonTenantAccessRevoked, FailureStageAuthorization, false, RemediationOwnerProductUser, "activation tenant context is required")
	}
	state, ok, err := s.stateStore.GetActivationStateForPrincipalTenant(ctx, principalID, tenantID)
	if err != nil {
		return State{}, err
	}
	if ok {
		return state, nil
	}
	now := s.now()
	return State{
		ActivationID:        stableActivationID("act", principalID, tenantID),
		PrincipalID:         principalID,
		TenantID:            tenantID,
		EnvironmentScope:    s.environmentScope,
		Status:              StatusNotStarted,
		CurrentStepID:       StepResolvePersonalTenant,
		CompletedStepIDs:    []string{},
		BlockingReasonCodes: []ReasonCode{},
		ReadinessItems:      []ReadinessItem{},
		FirstAction:         DefaultTestChatFirstAction(false, []string{"tenant-access"}),
		CreatedAt:           now,
		UpdatedAt:           now,
		LastEvaluatedAt:     now,
	}, nil
}

func (s *Service) resolvePersonalTenant(ctx context.Context, principal identity.Principal, token identity.TokenAuthority, now time.Time) (identity.Tenant, error) {
	if principal.DefaultTenantID != "" {
		tenant, ok, err := s.identity.GetTenant(ctx, principal.DefaultTenantID)
		if err != nil {
			return identity.Tenant{}, err
		}
		if ok && tenant.TenantKind == identity.TenantKindPersonal && tenant.Status == identity.StatusActive {
			if err := s.ensurePersonalTenantAccess(ctx, principal, tenant, token, now); err != nil {
				return identity.Tenant{}, err
			}
			return tenant, nil
		}
	}
	tenants, err := s.identity.ListTenants(ctx, identity.TenantFilter{TenantKind: identity.TenantKindPersonal, Status: identity.StatusActive, Limit: 1000})
	if err != nil {
		return identity.Tenant{}, err
	}
	for _, tenant := range tenants {
		if tenant.DefaultOwnerPrincipalID == principal.PrincipalID {
			if err := s.ensurePersonalTenantAccess(ctx, principal, tenant, token, now); err != nil {
				return identity.Tenant{}, err
			}
			return tenant, nil
		}
	}
	tenant := identity.Tenant{
		TenantID:                stableActivationID("ten_personal", principal.PrincipalID),
		TenantKind:              identity.TenantKindPersonal,
		DisplayName:             "Personal tenant",
		Status:                  identity.StatusActive,
		CreatedAt:               now,
		UpdatedAt:               now,
		CreatedByPrincipalID:    principal.PrincipalID,
		DefaultOwnerPrincipalID: principal.PrincipalID,
	}
	if err := s.identity.UpsertTenant(ctx, tenant); err != nil {
		return identity.Tenant{}, err
	}
	if err := s.ensurePersonalTenantAccess(ctx, principal, tenant, token, now); err != nil {
		return identity.Tenant{}, err
	}
	return tenant, nil
}

func (s *Service) ensurePersonalTenantAccess(ctx context.Context, principal identity.Principal, tenant identity.Tenant, token identity.TokenAuthority, now time.Time) error {
	memberships, err := s.identity.ListMemberships(ctx, identity.MembershipFilter{TenantID: tenant.TenantID, Limit: 1000})
	if err != nil {
		return err
	}
	hasMembership := false
	for _, membership := range memberships {
		if membership.PrincipalID != principal.PrincipalID {
			continue
		}
		if membership.Status != identity.StatusActive {
			return activationError(ReasonTenantAccessRevoked, FailureStageAuthorization, false, RemediationOwnerProductUser, "activation tenant access is revoked")
		}
		hasMembership = true
		break
	}
	if !hasMembership {
		acceptedAt := now
		if err := s.identity.UpsertMembership(ctx, identity.Membership{
			MembershipID: stableActivationID("mem", principal.PrincipalID, tenant.TenantID),
			TenantID:     tenant.TenantID,
			PrincipalID:  principal.PrincipalID,
			Role:         identity.RoleOwner,
			Status:       identity.StatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
			AcceptedAt:   &acceptedAt,
		}); err != nil {
			return err
		}
	}
	if strings.TrimSpace(token.TokenID) == "" {
		return nil
	}
	grants, err := s.identity.ListTokenTenantGrants(ctx, token.TokenID)
	if err != nil {
		return err
	}
	for _, grant := range grants {
		if grant.TenantID != tenant.TenantID {
			continue
		}
		if grant.Status != identity.StatusActive {
			return activationError(ReasonTenantAccessRevoked, FailureStageAuthorization, false, RemediationOwnerProductUser, "activation token tenant grant is revoked")
		}
		if grant.Status == identity.StatusActive {
			return nil
		}
	}
	return s.identity.UpsertTokenTenantGrant(ctx, identity.TokenTenantGrant{
		GrantID:              stableActivationID("grant", token.TokenID, tenant.TenantID),
		TokenID:              token.TokenID,
		TenantID:             tenant.TenantID,
		IsDefault:            true,
		Status:               identity.StatusActive,
		CreatedAt:            now,
		UpdatedAt:            now,
		GrantedByPrincipalID: principal.PrincipalID,
	})
}

func activationError(reason ReasonCode, stage FailureStage, retryable bool, owner RemediationOwner, message string) error {
	return &Error{
		ReasonCode:       reason,
		Stage:            stage,
		Retryable:        retryable,
		RemediationOwner: owner,
		Message:          message,
	}
}

func stableActivationID(prefix string, parts ...string) string {
	cleaned := make([]string, 0, len(parts)+1)
	cleaned = append(cleaned, prefix)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
				return r
			}
			return '_'
		}, part)
		part = strings.Trim(part, "_")
		if part != "" {
			cleaned = append(cleaned, strings.ToLower(part))
		}
	}
	return fmt.Sprintf("%s", strings.Join(cleaned, "_"))
}

type auditRecord struct {
	EventKind         string
	ActivationID      string
	TenantID          string
	PrincipalID       string
	TokenID           string
	Outcome           string
	ReasonCode        ReasonCode
	Stage             FailureStage
	FromStatus        Status
	ToStatus          Status
	Retryable         bool
	RemediationOwner  RemediationOwner
	TestChat          *TestChatMetadata
	CompletedStepIDs  []string
	ReadinessItemIDs  []string
	QuotaBaselineStat string
}
