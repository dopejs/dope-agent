package setupwizard

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/secrets"
)

type Store interface {
	SaveSetupSession(ctx context.Context, session SetupSession) error
	GetSetupSession(ctx context.Context, tenantID, sessionID string) (SetupSession, bool, error)
	ListSetupSessions(ctx context.Context, tenantID string) ([]SetupSession, error)
	AppendSetupAttempt(ctx context.Context, attempt SetupAttempt) error
	ListSetupAttempts(ctx context.Context, tenantID, sessionID string) ([]SetupAttempt, error)
}

type SecretManager interface {
	Create(ctx context.Context, input secrets.CreateInput) (secrets.TenantSecret, error)
	Rotate(ctx context.Context, input secrets.RotateInput) (secrets.TenantSecret, error)
	Get(ctx context.Context, tenantID, secretRef string) (secrets.TenantSecret, error)
	Disable(ctx context.Context, input secrets.DisableInput) (secrets.TenantSecret, error)
}

type DiagnosticProbe interface {
	ProbeSetup(ctx context.Context, session SetupSession, operation SetupOperation) (SetupDiagnosticProbeResult, error)
}

type SubmittedSecretDiagnosticProbe interface {
	ProbeSubmittedSecret(ctx context.Context, session SetupSession, input SubmitSecretInput) (SetupDiagnosticProbeResult, error)
}

type AuditRecorder interface {
	RecordSetupAudit(ctx context.Context, record SetupAuditRecord) (string, error)
}

type SubmittedSecretRecorder interface {
	RecordSubmittedSecretSetup(ctx context.Context, session SetupSession, input SubmitSecretInput) error
}

type OAuthCallbackRecorder interface {
	RecordOAuthSetup(ctx context.Context, session SetupSession, input OAuthCallbackInput) error
}

type OAuthStartURLProvider interface {
	AuthorizationURL(ctx context.Context, session SetupSession, input OAuthStartInput, defaultURL string) (string, error)
}

type ServiceDependencies struct {
	Store                   Store
	Secrets                 SecretManager
	Diagnostics             DiagnosticProbe
	Audit                   AuditRecorder
	SubmittedSecretRecorder SubmittedSecretRecorder
	OAuthCallbackRecorder   OAuthCallbackRecorder
	OAuthStartURLProvider   OAuthStartURLProvider
	Now                     func() time.Time
}

type Service struct {
	store                   Store
	secrets                 SecretManager
	diagnostics             DiagnosticProbe
	audit                   AuditRecorder
	submittedSecretRecorder SubmittedSecretRecorder
	oauthCallbackRecorder   OAuthCallbackRecorder
	oauthStartURLProvider   OAuthStartURLProvider
	now                     func() time.Time
}

func NewService(deps ServiceDependencies) *Service {
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	store := deps.Store
	if store == nil {
		store = NewMemoryStore()
	}
	diagnostics := deps.Diagnostics
	if diagnostics == nil {
		diagnostics = DefaultDiagnosticProbe{Secrets: deps.Secrets}
	}
	return &Service{store: store, secrets: deps.Secrets, diagnostics: diagnostics, audit: deps.Audit, submittedSecretRecorder: deps.SubmittedSecretRecorder, oauthCallbackRecorder: deps.OAuthCallbackRecorder, oauthStartURLProvider: deps.OAuthStartURLProvider, now: now}
}

type ListTargetsInput struct {
	TenantContext identity.TenantContext
}

type StartInput struct {
	TenantContext identity.TenantContext
	TargetID      string
	SetupStyle    SetupStyle
	Source        string
}

type SubmitSecretInput struct {
	TenantContext identity.TenantContext
	SessionID     string
	SecretRef     string
	Value         string
	DisplayName   string
	ResourceRefs  []ResourceRef
}

type OAuthStartInput struct {
	TenantContext identity.TenantContext
	SessionID     string
	RedirectRoute string
}

type OAuthStartResult struct {
	Session          SetupSession `json:"session"`
	AuthorizationURL string       `json:"authorizationUrl"`
	StateRef         string       `json:"state"`
}

type OAuthCallbackInput struct {
	TenantContext identity.TenantContext
	SessionID     string
	State         string
	Result        OAuthResult
	AccountLabel  string
	Code          string
	RedirectURI   string
}

type ReplaceInput struct {
	TenantContext identity.TenantContext
	SessionID     string
}

type DisableInput struct {
	TenantContext  identity.TenantContext
	SessionID      string
	DisabledReason string
}

func (s *Service) ListTargets(ctx context.Context, input ListTargetsInput) ([]SetupTarget, error) {
	if err := RequireInspection(input.TenantContext); err != nil {
		return nil, err
	}
	targets := CatalogTargets(input.TenantContext.TenantID)
	sessions, err := s.store.ListSetupSessions(ctx, input.TenantContext.TenantID)
	if err != nil {
		return nil, err
	}
	byTarget := make(map[string]SetupSession, len(sessions))
	for _, session := range sessions {
		byTarget[session.TargetID] = session
	}
	for i := range targets {
		if session, ok := byTarget[targets[i].TargetID]; ok {
			targets[i].CurrentSessionID = session.SetupSessionID
			targets[i].CurrentState = session.State
			targets[i].DiagnosticResultID = session.DiagnosticResultID
		}
	}
	return targets, nil
}

func (s *Service) ListSessions(ctx context.Context, tenantContext identity.TenantContext) ([]SetupSession, error) {
	if err := RequireInspection(tenantContext); err != nil {
		return nil, err
	}
	return s.store.ListSetupSessions(ctx, tenantContext.TenantID)
}

func (s *Service) Get(ctx context.Context, tenantContext identity.TenantContext, sessionID string) (SetupSession, error) {
	if err := RequireInspection(tenantContext); err != nil {
		return SetupSession{}, err
	}
	session, ok, err := s.store.GetSetupSession(ctx, tenantContext.TenantID, strings.TrimSpace(sessionID))
	if err != nil {
		return SetupSession{}, err
	}
	if !ok {
		return SetupSession{}, ErrSessionNotFound
	}
	return session, nil
}

func (s *Service) Start(ctx context.Context, input StartInput) (SetupSession, error) {
	if err := RequireMutation(input.TenantContext); err != nil {
		return SetupSession{}, err
	}
	target, ok := TargetByID(input.TenantContext.TenantID, input.TargetID)
	if !ok || target.SupportStatus != SupportStatusSupported {
		return SetupSession{}, ErrUnsupportedTarget
	}
	if input.SetupStyle != "" && input.SetupStyle != target.SetupStyle {
		return SetupSession{}, fmt.Errorf("setup style %s does not match target %s", input.SetupStyle, target.SetupStyle)
	}
	now := s.now()
	session := SetupSession{
		SetupSessionID:   sessionID(input.TenantContext.TenantID, target.TargetID, target.SetupStyle),
		TenantID:         input.TenantContext.TenantID,
		ActorPrincipalID: input.TenantContext.PrincipalID,
		TargetID:         target.TargetID,
		TargetKind:       target.TargetKind,
		SetupStyle:       target.SetupStyle,
		State:            StateInProgress,
		Retryable:        true,
		RemediationOwner: OwnerProductUser,
		SafeUseMode:      SafeUseBlocked,
		RedactionStatus:  RedactionRedacted,
		CreatedAt:        now,
		UpdatedAt:        now,
		LastTransitionAt: now,
	}
	if existing, ok, err := s.store.GetSetupSession(ctx, session.TenantID, session.SetupSessionID); err != nil {
		return SetupSession{}, err
	} else if ok {
		session.CreatedAt = existing.CreatedAt
		session.ResourceRefs = append([]ResourceRef(nil), existing.ResourceRefs...)
	}
	return s.transition(ctx, session, OperationStart, StateInProgress, "", nil)
}

func (s *Service) transition(ctx context.Context, session SetupSession, op SetupOperation, to SetupState, reason string, evidence map[string]string) (SetupSession, error) {
	from := session.State
	now := s.now()
	session.State = to
	session.ReasonCode = reason
	session.RedactedEvidence = cloneStringMap(evidence)
	session.UpdatedAt = now
	session.LastTransitionAt = now
	session.RedactionStatus = firstRedaction(session.RedactionStatus)
	session.SafeUseMode = safeUseForState(session)
	session.Retryable = retryableForState(to)
	session.RemediationOwner = remediationOwnerForState(to, reason)
	if to == StateReady {
		session.ReasonCode = ReasonHealthy
	}
	if to == StateReady && session.DiagnosticResultID == "" {
		return SetupSession{}, ErrDiagnosticLinkNeeded
	}
	attempt := SetupAttempt{
		AttemptID:          attemptID(session.SetupSessionID, op, now),
		SetupSessionID:     session.SetupSessionID,
		TenantID:           session.TenantID,
		ActorPrincipalID:   session.ActorPrincipalID,
		Operation:          op,
		FromState:          from,
		ToState:            to,
		ReasonCode:         session.ReasonCode,
		RedactedEvidence:   cloneStringMap(evidence),
		ResourceRefs:       append([]ResourceRef(nil), session.ResourceRefs...),
		RedactionStatus:    session.RedactionStatus,
		DiagnosticResultID: session.DiagnosticResultID,
		CreatedAt:          now,
	}
	session.CurrentAttemptID = attempt.AttemptID
	if ContainsForbiddenEvidence(session, nil) || ContainsForbiddenEvidence(attempt, nil) {
		session = failClosed(session, ReasonRedactionFailedClosed)
		attempt.ToState = session.State
		attempt.ReasonCode = session.ReasonCode
		attempt.RedactionStatus = session.RedactionStatus
	}
	if err := s.store.SaveSetupSession(ctx, session); err != nil {
		return SetupSession{}, err
	}
	if err := s.store.AppendSetupAttempt(ctx, attempt); err != nil {
		return SetupSession{}, err
	}
	if s.audit != nil {
		auditRecord := AuditRecordForAttempt(session, attempt)
		auditID, err := s.audit.RecordSetupAudit(ctx, auditRecord)
		if err != nil {
			return SetupSession{}, err
		}
		if auditID != "" {
			session.LastTransitionAuditID = auditID
			if err := s.store.SaveSetupSession(ctx, session); err != nil {
				return SetupSession{}, err
			}
		}
	}
	return session, nil
}

func (s *Service) loadForMutation(ctx context.Context, tc identity.TenantContext, sessionIDValue string) (SetupSession, error) {
	if err := RequireMutation(tc); err != nil {
		return SetupSession{}, err
	}
	if strings.TrimSpace(sessionIDValue) == "" {
		return SetupSession{}, ErrSessionRequired
	}
	session, ok, err := s.store.GetSetupSession(ctx, tc.TenantID, strings.TrimSpace(sessionIDValue))
	if err != nil {
		return SetupSession{}, err
	}
	if !ok {
		return SetupSession{}, ErrSessionNotFound
	}
	session.ActorPrincipalID = firstNonEmpty(tc.PrincipalID, session.ActorPrincipalID)
	return session, nil
}

type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]SetupSession
	attempts map[string][]SetupAttempt
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]SetupSession),
		attempts: make(map[string][]SetupAttempt),
	}
}

func (s *MemoryStore) SaveSetupSession(_ context.Context, session SetupSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.TenantID+"::"+session.SetupSessionID] = cloneSession(session)
	return nil
}

func (s *MemoryStore) GetSetupSession(_ context.Context, tenantID, sessionID string) (SetupSession, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[strings.TrimSpace(tenantID)+"::"+strings.TrimSpace(sessionID)]
	return cloneSession(session), ok, nil
}

func (s *MemoryStore) ListSetupSessions(_ context.Context, tenantID string) ([]SetupSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]SetupSession, 0)
	for _, item := range s.sessions {
		if strings.TrimSpace(item.TenantID) == strings.TrimSpace(tenantID) {
			items = append(items, cloneSession(item))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
	return items, nil
}

func (s *MemoryStore) AppendSetupAttempt(_ context.Context, attempt SetupAttempt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := attempt.TenantID + "::" + attempt.SetupSessionID
	s.attempts[key] = append(s.attempts[key], cloneAttempt(attempt))
	return nil
}

func (s *MemoryStore) ListSetupAttempts(_ context.Context, tenantID, sessionID string) ([]SetupAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := append([]SetupAttempt(nil), s.attempts[strings.TrimSpace(tenantID)+"::"+strings.TrimSpace(sessionID)]...)
	for i := range items {
		items[i] = cloneAttempt(items[i])
	}
	return items, nil
}

func mapSecretError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, secrets.ErrSecretNotFound) {
		return err
	}
	return err
}
