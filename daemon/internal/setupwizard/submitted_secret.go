package setupwizard

import (
	"context"
	"errors"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/secrets"
)

func (s *Service) SubmitSecret(ctx context.Context, input SubmitSecretInput) (SetupSession, error) {
	session, err := s.loadForMutation(ctx, input.TenantContext, input.SessionID)
	if err != nil {
		return SetupSession{}, err
	}
	if session.SetupStyle != SetupStyleSubmittedSecret {
		return SetupSession{}, ErrUnsupportedTarget
	}
	secretRef := strings.TrimSpace(input.SecretRef)
	if secretRef == "" {
		return SetupSession{}, ErrSecretRefRequired
	}
	if strings.TrimSpace(input.Value) == "" {
		return SetupSession{}, ErrSecretValueRequired
	}
	if ContainsForbiddenEvidence(map[string]string{"displayName": input.DisplayName, "secretRef": secretRef}, []string{input.Value}) {
		session = failClosed(session, ReasonRedactionFailedClosed)
		return s.transition(ctx, session, OperationSubmitSecret, session.State, session.ReasonCode, session.RedactedEvidence)
	}
	versionID := "redacted_version"
	if s.secrets != nil {
		secret, err := s.secrets.Get(ctx, session.TenantID, secretRef)
		if errors.Is(err, secrets.ErrSecretNotFound) {
			secret, err = s.secrets.Create(ctx, secrets.CreateInput{
				TenantID:    session.TenantID,
				SecretRef:   secretRef,
				DisplayName: input.DisplayName,
				Value:       input.Value,
				Document:    map[string]any{"source": "setup_wizard", "targetId": session.TargetID},
			})
		} else if err == nil {
			secret, err = s.secrets.Rotate(ctx, secrets.RotateInput{TenantID: session.TenantID, SecretRef: secretRef, Value: input.Value})
		}
		if err != nil {
			return SetupSession{}, mapSecretError(err)
		}
		versionID = secret.ActiveVersionID
	}
	session.ResourceRefs = upsertResourceRef(session.ResourceRefs, ResourceRef{Kind: "tenant_secret", ID: secretRef, Route: "/v1/tenant-secrets/" + secretRef})
	for _, ref := range input.ResourceRefs {
		if strings.TrimSpace(ref.Kind) == "" || strings.TrimSpace(ref.ID) == "" {
			continue
		}
		session.ResourceRefs = upsertResourceRef(session.ResourceRefs, ref)
	}
	session.RedactedEvidence = RedactedSecretEvidence(secretRef, input.DisplayName)
	session.RedactedEvidence["secretVersionId"] = versionID
	session, state, reason, err := s.probeSubmittedSecretReadiness(ctx, session, input)
	if err != nil {
		return SetupSession{}, err
	}
	session, err = s.transition(ctx, session, OperationSubmitSecret, state, reason, session.RedactedEvidence)
	if err != nil {
		return SetupSession{}, err
	}
	if s.submittedSecretRecorder != nil {
		if err := s.submittedSecretRecorder.RecordSubmittedSecretSetup(ctx, session, input); err != nil {
			return SetupSession{}, err
		}
	}
	return session, nil
}
