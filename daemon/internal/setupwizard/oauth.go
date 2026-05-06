package setupwizard

import (
	"context"
	"strings"
)

func (s *Service) StartOAuth(ctx context.Context, input OAuthStartInput) (OAuthStartResult, error) {
	session, err := s.loadForMutation(ctx, input.TenantContext, input.SessionID)
	if err != nil {
		return OAuthStartResult{}, err
	}
	if session.SetupStyle != SetupStyleOAuth {
		return OAuthStartResult{}, ErrUnsupportedTarget
	}
	stateRef := oauthStateRef(session)
	session.OAuthStateRef = stateRef
	evidence := map[string]string{
		"redactionRule": "oauth_start_metadata_only",
		"redirectRoute": strings.TrimSpace(input.RedirectRoute),
	}
	updated, err := s.transition(ctx, session, OperationOAuthStart, StateInProgress, "", evidence)
	if err != nil {
		return OAuthStartResult{}, err
	}
	return OAuthStartResult{
		Session:          updated,
		AuthorizationURL: "https://oauth.test/authorize?state=" + stateRef,
		StateRef:         stateRef,
	}, nil
}

func (s *Service) CompleteOAuth(ctx context.Context, input OAuthCallbackInput) (SetupSession, error) {
	session, err := s.loadForMutation(ctx, input.TenantContext, input.SessionID)
	if err != nil {
		return SetupSession{}, err
	}
	if session.SetupStyle != SetupStyleOAuth {
		return SetupSession{}, ErrUnsupportedTarget
	}
	if strings.TrimSpace(input.State) == "" {
		return SetupSession{}, ErrOAuthStateRequired
	}
	if session.OAuthStateRef != "" && strings.TrimSpace(input.State) != session.OAuthStateRef {
		input.Result = OAuthResultTenantMismatch
	}
	state, reason := mapOAuthResult(input.Result)
	evidence := RedactedOAuthEvidence(input.Result, input.AccountLabel)
	session.RedactedEvidence = evidence
	session.ResourceRefs = upsertResourceRef(session.ResourceRefs, ResourceRef{Kind: "provider_auth_state", ID: session.TargetID})
	if state == StateReady || state == StateDegraded {
		var err error
		session, state, reason, err = s.probeReadiness(ctx, session, OperationOAuthCallback)
		if err != nil {
			return SetupSession{}, err
		}
	}
	return s.transition(ctx, session, OperationOAuthCallback, state, reason, evidence)
}

func mapOAuthResult(result OAuthResult) (SetupState, string) {
	switch result {
	case OAuthResultCompleted:
		return StateReady, ReasonHealthy
	case OAuthResultDenied:
		return StateActionRequired, ReasonOAuthDenied
	case OAuthResultAbandoned:
		return StateCancelled, ReasonOAuthAbandoned
	case OAuthResultExpired:
		return StateActionRequired, ReasonOAuthExpired
	case OAuthResultReplay:
		return StateActionRequired, ReasonOAuthReplay
	case OAuthResultTenantMismatch:
		return StateActionRequired, ReasonTenantMismatch
	case OAuthResultProviderError:
		return StateUnavailable, ReasonProviderUnavailable
	default:
		return StateActionRequired, ReasonOAuthDenied
	}
}
