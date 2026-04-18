package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	baseconnectors "github.com/dopejs/dope-agent/daemon/internal/connectors"
	"github.com/dopejs/dope-agent/daemon/internal/events"
	"github.com/dopejs/dope-agent/daemon/internal/im"
	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

type Config struct {
	Enabled           bool
	ConnectorID       string
	DisplayName       string
	DeliveryMode      string
	BotToken          string
	RequireMention    bool
	RespondInDM       bool
	AllowedGuildIDs   []string
	AllowedChannelIDs []string
}

type Transport interface {
	Start(ctx context.Context, handle func(context.Context, imtypes.InboundMessage)) error
	SendReply(ctx context.Context, reply imtypes.OutboundReply) (imtypes.SentReply, error)
	Close(ctx context.Context) error
}

var newGatewayTransport = func(cfg Config) (Transport, error) {
	return NewGatewayTransport(cfg)
}

type Runtime struct {
	cfg        Config
	logger     *slog.Logger
	supervisor *baseconnectors.Supervisor
	loop       *im.MessageLoop
	store      *store.SQLiteStore
	eventBus   *events.Bus
	transport  Transport

	mu      sync.Mutex
	started bool
}

func NewRuntime(cfg Config, logger *slog.Logger, supervisor *baseconnectors.Supervisor, loop *im.MessageLoop, sqliteStore *store.SQLiteStore, eventBus *events.Bus, transport Transport) (*Runtime, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if strings.TrimSpace(cfg.ConnectorID) == "" {
		return nil, fmt.Errorf("discord connector id is required")
	}
	if strings.TrimSpace(cfg.DisplayName) == "" {
		return nil, fmt.Errorf("discord display name is required")
	}
	if mode := strings.TrimSpace(cfg.DeliveryMode); mode == "" {
		cfg.DeliveryMode = "gateway"
	} else if mode != "gateway" {
		return nil, fmt.Errorf("unsupported discord delivery mode: %s", mode)
	}
	if strings.TrimSpace(cfg.BotToken) == "" {
		return nil, fmt.Errorf("discord bot token is required")
	}
	if supervisor == nil || loop == nil {
		return nil, fmt.Errorf("discord connector dependencies are not configured")
	}
	if transport == nil {
		var err error
		transport, err = newGatewayTransport(cfg)
		if err != nil {
			return nil, err
		}
	}

	return &Runtime{
		cfg:        cfg,
		logger:     logger,
		supervisor: supervisor,
		loop:       loop,
		store:      sqliteStore,
		eventBus:   eventBus,
		transport:  transport,
	}, nil
}

func (r *Runtime) Start(ctx context.Context) error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return nil
	}
	r.started = true
	r.mu.Unlock()

	connector, _, err := r.supervisor.Register(baseconnectors.RegisterInput{
		ConnectorID: r.cfg.ConnectorID,
		Kind:        "discord",
		DisplayName: r.cfg.DisplayName,
	})
	if err != nil {
		return err
	}
	if err := r.persistConnector(ctx, connector); err != nil {
		return err
	}

	if err := r.transport.Start(ctx, r.handleInbound); err != nil {
		r.started = false
		if failed, reportErr := r.supervisor.ReportFailure(r.cfg.ConnectorID, baseconnectors.ReportFailureInput{Reason: err.Error()}); reportErr == nil {
			_ = r.persistConnector(ctx, failed)
			_, _ = r.publishEvent(ctx, "connector.failed", map[string]any{
				"kind":         failed.Kind,
				"status":       failed.Status,
				"deliveryMode": r.cfg.DeliveryMode,
				"error":        err.Error(),
				"errorClass":   classifyDiscordError(err),
			})
		}
		return err
	}

	connector, err = r.supervisor.ReportHealth(r.cfg.ConnectorID, baseconnectors.ReportHealthInput{
		Status: baseconnectors.StatusHealthy,
	})
	if err != nil {
		return err
	}
	if err := r.persistConnector(ctx, connector); err != nil {
		return err
	}
	_, _ = r.publishEvent(ctx, "connector.healthy", map[string]any{
		"kind":         connector.Kind,
		"status":       connector.Status,
		"deliveryMode": r.cfg.DeliveryMode,
	})
	return nil
}

func (r *Runtime) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return nil
	}
	r.started = false
	r.mu.Unlock()

	return r.transport.Close(ctx)
}

func (r *Runtime) handleInbound(ctx context.Context, inbound imtypes.InboundMessage) {
	if !r.shouldHandle(inbound) {
		return
	}
	if connector, err := r.supervisor.ReportHealth(r.cfg.ConnectorID, baseconnectors.ReportHealthInput{Status: baseconnectors.StatusHealthy}); err == nil {
		_ = r.persistConnector(ctx, connector)
	}

	if r.logger != nil {
		r.logger.Info("discord inbound message accepted", "connector_id", r.cfg.ConnectorID, "message_id", inbound.ExternalMessageID)
	}

	result, err := r.loop.ProcessSingleTurn(ctx, baseconnectors.Connector{
		ConnectorID: r.cfg.ConnectorID,
		Kind:        "discord",
		DisplayName: r.cfg.DisplayName,
		Status:      baseconnectors.StatusHealthy,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}, inbound, r.transport)
	if err != nil {
		if r.logger != nil {
			r.logger.Error("discord message loop failed", "connector_id", r.cfg.ConnectorID, "message_id", inbound.ExternalMessageID, "error", err.Error())
		}
		return
	}
	if result.Duplicate && r.logger != nil {
		r.logger.Info("discord duplicate message ignored", "connector_id", r.cfg.ConnectorID, "message_id", inbound.ExternalMessageID)
	}
}

func (r *Runtime) shouldHandle(inbound imtypes.InboundMessage) bool {
	if inbound.Direct {
		return r.cfg.RespondInDM
	}
	if len(r.cfg.AllowedGuildIDs) > 0 && !contains(r.cfg.AllowedGuildIDs, inbound.GuildID) {
		return false
	}
	if len(r.cfg.AllowedChannelIDs) > 0 && !contains(r.cfg.AllowedChannelIDs, inbound.ChannelID) {
		return false
	}
	if r.cfg.RequireMention && !inbound.Mentioned {
		return false
	}
	return true
}

func (r *Runtime) persistConnector(ctx context.Context, connector baseconnectors.Connector) error {
	if r.store == nil {
		return nil
	}
	return r.store.UpsertConnector(ctx, connector)
}

func (r *Runtime) publishEvent(ctx context.Context, name string, payload map[string]any) (events.Event, error) {
	if r.eventBus == nil {
		return events.Event{}, nil
	}
	event := events.Event{
		Category: "connector",
		Name:     name,
		Scope: events.Scope{
			ConnectorID: r.cfg.ConnectorID,
		},
		Resource: events.Resource{
			Kind: "connector",
			ID:   r.cfg.ConnectorID,
		},
		Payload: payload,
	}
	if r.store != nil {
		persisted, err := r.store.AppendEvent(ctx, event)
		if err != nil {
			return events.Event{}, err
		}
		event = persisted
	}
	return r.eventBus.Publish(event), nil
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) == strings.TrimSpace(target) {
			return true
		}
	}
	return false
}

func classifyDiscordError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "401"),
		strings.Contains(message, "403"),
		strings.Contains(message, "unauthorized"),
		strings.Contains(message, "forbidden"),
		strings.Contains(message, "token"):
		return "auth_error"
	default:
		return "transport_error"
	}
}
