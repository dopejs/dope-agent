package store

import (
	"context"
	"testing"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/identity"
	"github.com/dopejs/dope-agent/daemon/internal/imtypes"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

func TestSQLiteStoreCreateConnectorMessageIfAbsentDeduplicatesByStandardIdentityWithinTenant(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	now := time.Now().UTC()
	first, created, err := store.CreateConnectorMessageIfAbsent(connectorIdentityCtx("ten_a"), imtypes.MessageRecord{
		DeliveryID:              "delivery_a1",
		ConnectorID:             "discord-main",
		Direction:               imtypes.DeliveryDirectionInbound,
		ExternalMessageID:       "transport_retry_1",
		ConnectorAccountID:      "acct_discord",
		ChannelOrConversationID: "channel_1",
		ProviderMessageID:       "provider_msg_1",
		EquivalentRuleID:        "discord_message_id",
		ChannelID:               "channel_1",
		Content:                 "hello",
		Status:                  imtypes.DeliveryStatusReceived,
		CreatedAt:               now,
		UpdatedAt:               now,
	})
	if err != nil {
		t.Fatalf("CreateConnectorMessageIfAbsent(first): %v", err)
	}
	if !created {
		t.Fatal("expected first insert to create a row")
	}

	second, created, err := store.CreateConnectorMessageIfAbsent(connectorIdentityCtx("ten_a"), imtypes.MessageRecord{
		DeliveryID:              "delivery_a2",
		ConnectorID:             "discord-main",
		Direction:               imtypes.DeliveryDirectionInbound,
		ExternalMessageID:       "transport_retry_2",
		ConnectorAccountID:      "acct_discord",
		ChannelOrConversationID: "channel_1",
		ProviderMessageID:       "provider_msg_1",
		EquivalentRuleID:        "discord_message_id",
		ChannelID:               "channel_1",
		Content:                 "hello duplicate",
		Status:                  imtypes.DeliveryStatusReceived,
		CreatedAt:               now.Add(time.Second),
		UpdatedAt:               now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("CreateConnectorMessageIfAbsent(second): %v", err)
	}
	if created {
		t.Fatal("expected duplicate standard identity to return existing row")
	}
	if second.DeliveryID != first.DeliveryID {
		t.Fatalf("duplicate returned delivery %s, want %s", second.DeliveryID, first.DeliveryID)
	}

	_, created, err = store.CreateConnectorMessageIfAbsent(connectorIdentityCtx("ten_b"), imtypes.MessageRecord{
		DeliveryID:              "delivery_b1",
		ConnectorID:             "discord-main",
		Direction:               imtypes.DeliveryDirectionInbound,
		ExternalMessageID:       "transport_retry_3",
		ConnectorAccountID:      "acct_discord",
		ChannelOrConversationID: "channel_1",
		ProviderMessageID:       "provider_msg_1",
		EquivalentRuleID:        "discord_message_id",
		ChannelID:               "channel_1",
		Content:                 "hello other tenant",
		Status:                  imtypes.DeliveryStatusReceived,
		CreatedAt:               now,
		UpdatedAt:               now,
	})
	if err != nil {
		t.Fatalf("CreateConnectorMessageIfAbsent(other tenant): %v", err)
	}
	if !created {
		t.Fatal("expected same provider message identity in another tenant to create a separate row")
	}
}

func TestSQLiteStoreConnectorMessageExternalIDLookupIsTenantScoped(t *testing.T) {
	t.Parallel()

	store, err := NewSQLiteStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	now := time.Now().UTC()
	_, created, err := store.CreateConnectorMessageIfAbsent(connectorIdentityCtx("ten_a"), imtypes.MessageRecord{
		DeliveryID:        "delivery_external_a",
		ConnectorID:       "discord-main",
		Direction:         imtypes.DeliveryDirectionInbound,
		ExternalMessageID: "transport_shared",
		ChannelID:         "channel_a",
		Content:           "tenant a",
		Status:            imtypes.DeliveryStatusReceived,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		t.Fatalf("CreateConnectorMessageIfAbsent(tenant a): %v", err)
	}
	if !created {
		t.Fatal("expected tenant a external message insert to create a row")
	}

	_, created, err = store.CreateConnectorMessageIfAbsent(connectorIdentityCtx("ten_b"), imtypes.MessageRecord{
		DeliveryID:        "delivery_external_b",
		ConnectorID:       "discord-main",
		Direction:         imtypes.DeliveryDirectionInbound,
		ExternalMessageID: "transport_shared",
		ChannelID:         "channel_b",
		Content:           "tenant b",
		Status:            imtypes.DeliveryStatusReceived,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		t.Fatalf("CreateConnectorMessageIfAbsent(tenant b): %v", err)
	}
	if !created {
		t.Fatal("expected same external id in another tenant to create a separate row")
	}

	recordA, ok, err := store.GetConnectorMessageByExternalID(connectorIdentityCtx("ten_a"), "discord-main", imtypes.DeliveryDirectionInbound, "transport_shared")
	if err != nil || !ok {
		t.Fatalf("GetConnectorMessageByExternalID(tenant a) ok=%v err=%v", ok, err)
	}
	if recordA.DeliveryID != "delivery_external_a" {
		t.Fatalf("tenant a lookup returned %s", recordA.DeliveryID)
	}
	recordB, ok, err := store.GetConnectorMessageByExternalID(connectorIdentityCtx("ten_b"), "discord-main", imtypes.DeliveryDirectionInbound, "transport_shared")
	if err != nil || !ok {
		t.Fatalf("GetConnectorMessageByExternalID(tenant b) ok=%v err=%v", ok, err)
	}
	if recordB.DeliveryID != "delivery_external_b" {
		t.Fatalf("tenant b lookup returned %s", recordB.DeliveryID)
	}
}

func connectorIdentityCtx(tenantID string) context.Context {
	return tenantctx.WithContext(context.Background(), identity.TenantContext{
		TenantID:    tenantID,
		PrincipalID: "prn_" + tenantID,
	})
}
