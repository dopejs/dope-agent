package reminders

import (
	"context"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

func refreshFollowUpLink(ctx context.Context, sqliteStore *store.SQLiteStore, environmentScope string, link *FollowUpLink) (*FollowUpLink, error) {
	if sqliteStore == nil || link == nil {
		return cloneFollowUpLink(link), nil
	}
	out := cloneFollowUpLink(link)
	now := time.Now().UTC()
	out.LastCheckedAt = &now

	var stale bool
	switch out.LinkKind {
	case FollowUpLinkKindRun:
		items, err := sqliteStore.ListRuns(ctx)
		if err != nil {
			return nil, err
		}
		stale = true
		for _, item := range items {
			if item.RunID == out.SourceID {
				stale = false
				break
			}
		}
	case FollowUpLinkKindWorkflow:
		_, ok, err := sqliteStore.GetWorkflowByID(ctx, environmentScope, out.SourceID)
		if err != nil {
			return nil, err
		}
		stale = !ok
	case FollowUpLinkKindCalendarOperation:
		_, ok, err := sqliteStore.GetCalendarOperationByID(ctx, environmentScope, out.SourceID)
		if err != nil {
			return nil, err
		}
		stale = !ok
	case FollowUpLinkKindMailOperation:
		_, ok, err := sqliteStore.GetMailOperationByID(ctx, environmentScope, out.SourceID)
		if err != nil {
			return nil, err
		}
		stale = !ok
	default:
		stale = false
	}
	out.Stale = stale
	if stale && out.SourceDisplayState == "" {
		out.SourceDisplayState = "stale"
	}
	return out, nil
}
