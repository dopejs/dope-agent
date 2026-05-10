package matrix

import (
	"context"
	"errors"
	"strings"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
)

type SmokeStatus string

const (
	SmokePassed  SmokeStatus = "passed"
	SmokeFailed  SmokeStatus = "failed"
	SmokeSkipped SmokeStatus = "skipped"
)

type SmokeAuthorizationMode string

const (
	SmokeAuthorizationSafeLive    SmokeAuthorizationMode = "safe_live"
	SmokeAuthorizationFakeMatrix  SmokeAuthorizationMode = "fake_matrix"
	SmokeAuthorizationUnavailable SmokeAuthorizationMode = "unavailable"
)

type SmokeEvidence struct {
	SmokeEvidenceID     string
	TenantID            string
	ConnectorID         string
	HomeserverBindingID string
	Status              SmokeStatus
	AuthorizationMode   SmokeAuthorizationMode
	Owner               string
	Reason              string
	RemainingRisk       string
	ValidatedAt         time.Time
	RetentionExpiresAt  time.Time
	RedactionStatus     baseconnectors.RedactionStatus
	SafeEvidence        map[string]string
}

type SafeLiveSmokeInput struct {
	TenantID    string
	ConnectorID string
	Owner       string
	Now         time.Time
	Transport   interface {
		ValidateHomeserverBinding(context.Context, HomeserverBinding) (HomeserverBinding, error)
		ValidateRoutePolicy(context.Context, RoutePolicy) (RoutePolicy, error)
		SendReply(context.Context, imtypes.OutboundReply) (imtypes.SentReply, error)
	}
	Binding     HomeserverBinding
	RoutePolicy RoutePolicy
	SmokeRoomID string
}

func StructuredSkipSmokeEvidence(tenantID, connectorID, owner, reason string, now time.Time) SmokeEvidence {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return SmokeEvidence{
		SmokeEvidenceID:     "matrix_smoke_" + connectorID,
		TenantID:            tenantID,
		ConnectorID:         connectorID,
		HomeserverBindingID: "matrix_homeserver_" + connectorID,
		Status:              SmokeSkipped,
		AuthorizationMode:   SmokeAuthorizationUnavailable,
		Owner:               owner,
		Reason:              reason,
		RemainingRisk:       "No live Matrix hosted smoke was run; release review must consume this structured skip.",
		ValidatedAt:         now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
		RedactionStatus:     baseconnectors.RedactionStatusRedacted,
		SafeEvidence:        map[string]string{"policy": "structured_skip"},
	}
}

func ExecuteSafeLiveSmoke(ctx context.Context, input SafeLiveSmokeInput) (SmokeEvidence, error) {
	if input.Transport == nil {
		return SmokeEvidence{}, errors.New("matrix safe-live smoke transport is required")
	}
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	binding, err := input.Transport.ValidateHomeserverBinding(ctx, input.Binding)
	if err != nil {
		return SmokeEvidence{}, err
	}
	policy := input.RoutePolicy
	if strings.TrimSpace(policy.HomeserverBindingID) == "" {
		policy.HomeserverBindingID = binding.HomeserverBindingID
	}
	policy, err = input.Transport.ValidateRoutePolicy(ctx, policy)
	if err != nil {
		return SmokeEvidence{}, err
	}
	if !HasReadyRoutePolicy(policy) {
		return SmokeEvidence{}, errors.New("matrix route policy is not ready for safe-live smoke")
	}
	roomID := strings.TrimSpace(input.SmokeRoomID)
	if roomID == "" && len(policy.SelectedRooms) > 0 {
		roomID = policy.SelectedRooms[0].ConversationID
	}
	if roomID == "" {
		return SmokeEvidence{}, errors.New("matrix safe-live smoke room is required")
	}
	sent, err := input.Transport.SendReply(ctx, imtypes.OutboundReply{
		ConnectorID: input.ConnectorID,
		ChannelID:   roomID,
		Content:     "DopeAgent Matrix smoke validation",
	})
	if err != nil {
		return SmokeEvidence{}, err
	}
	bindingID := binding.HomeserverBindingID
	if strings.TrimSpace(bindingID) == "" {
		bindingID = "matrix_homeserver_" + strings.TrimSpace(input.ConnectorID)
	}
	return SmokeEvidence{
		SmokeEvidenceID:     "matrix_smoke_" + input.ConnectorID,
		TenantID:            input.TenantID,
		ConnectorID:         input.ConnectorID,
		HomeserverBindingID: bindingID,
		Status:              SmokePassed,
		AuthorizationMode:   SmokeAuthorizationSafeLive,
		Owner:               input.Owner,
		Reason:              "healthy",
		ValidatedAt:         now,
		RetentionExpiresAt:  now.Add(90 * 24 * time.Hour),
		RedactionStatus:     baseconnectors.RedactionStatusRedacted,
		SafeEvidence: map[string]string{
			"policy":  "safe_live_matrix_smoke",
			"eventId": sent.ExternalMessageID,
		},
	}, nil
}
