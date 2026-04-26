package tenancy

import (
	"context"
	"errors"

	"github.com/dopejs/dope-agent/daemon/internal/audit"
	"github.com/dopejs/dope-agent/daemon/internal/mail"
	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Mail is the tenant-aware accessor for mail_accounts, mail_operations,
// mail_artifacts.
type Mail struct {
	store   *store.SQLiteStore
	emitter *audit.Emitter
}

func NewMail(s *store.SQLiteStore, emitter *audit.Emitter) *Mail {
	return &Mail{store: s, emitter: emitter}
}

func (a *Mail) emit(ctx context.Context, surface, resourceKind string) {
	if a == nil || a.emitter == nil {
		return
	}
	_ = a.emitter.Emit(ctx, surface, resourceKind)
}

func (a *Mail) UpsertAccountForTenant(ctx context.Context, item mail.AccountProjection) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertMailAccount(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "mail_accounts", "mail_account_id", item.MailAccountID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertMailAccountForTenant", "mail_account")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Mail) UpsertOperationForTenant(ctx context.Context, item mail.Operation) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertMailOperation(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "mail_operations", "operation_id", item.OperationID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertMailOperationForTenant", "mail_operation")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}

func (a *Mail) UpsertArtifactForTenant(ctx context.Context, item mail.Artifact) error {
	tenantID, err := Require(ctx)
	if err != nil {
		return err
	}
	if err := a.store.UpsertMailArtifact(ctx, item); err != nil {
		return err
	}
	if err := a.store.BindRowTenant(ctx, "mail_artifacts", "artifact_id", item.ArtifactID, tenantID); err != nil {
		if errors.Is(err, store.ErrCrossTenantRow) {
			a.emit(ctx, "store:UpsertMailArtifactForTenant", "mail_artifact")
			return ErrCrossTenantWrite
		}
		return err
	}
	return nil
}
