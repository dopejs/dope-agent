package setupwizard

import (
	"context"
)

func (s *Service) Retry(ctx context.Context, input ReplaceInput) (SetupSession, error) {
	session, err := s.loadForMutation(ctx, input.TenantContext, input.SessionID)
	if err != nil {
		return SetupSession{}, err
	}
	return s.transition(ctx, session, OperationRetry, StateInProgress, "", map[string]string{"redactionRule": "retry_metadata_only"})
}

func (s *Service) Replace(ctx context.Context, input ReplaceInput) (SetupSession, error) {
	session, err := s.loadForMutation(ctx, input.TenantContext, input.SessionID)
	if err != nil {
		return SetupSession{}, err
	}
	session.ResourceRefs = append([]ResourceRef(nil), session.ResourceRefs...)
	return s.transition(ctx, session, OperationReplace, StateInProgress, "", map[string]string{"redactionRule": "replace_metadata_only"})
}

func (s *Service) Cancel(ctx context.Context, input ReplaceInput) (SetupSession, error) {
	session, err := s.loadForMutation(ctx, input.TenantContext, input.SessionID)
	if err != nil {
		return SetupSession{}, err
	}
	return s.transition(ctx, session, OperationCancel, StateCancelled, ReasonUserCancelled, map[string]string{"redactionRule": "cancel_metadata_only"})
}

func (s *Service) Disable(ctx context.Context, input DisableInput) (SetupSession, error) {
	session, err := s.loadForMutation(ctx, input.TenantContext, input.SessionID)
	if err != nil {
		return SetupSession{}, err
	}
	if s.secrets != nil && session.SetupStyle == SetupStyleSubmittedSecret {
		for _, ref := range session.ResourceRefs {
			if ref.Kind == "tenant_secret" && ref.ID != "" {
				_, _ = s.secrets.Disable(ctx, secretsDisableInput(session.TenantID, ref.ID, input.DisabledReason))
			}
		}
	}
	return s.transition(ctx, session, OperationDisable, StateDisabled, ReasonDisabledByUser, map[string]string{
		"redactionRule":  "disable_metadata_only",
		"disabledReason": input.DisabledReason,
	})
}
