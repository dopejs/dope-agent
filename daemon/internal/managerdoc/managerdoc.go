// Package managerdoc provides generic JSON-document persistence for the Roadmap 65-71 managers
// (triage, routine, webhook, catalog, execprofile, evidence) over the store's manager_documents
// table. It keeps each manager's persistence to a one-line write-through + a typed reload, with
// no per-manager SQL or marshal boilerplate.
package managerdoc

import (
	"context"
	"encoding/json"

	"github.com/dopejs/dope-agent/daemon/internal/store"
)

// Store is the subset of the SQLite store the document helpers use. *store.SQLiteStore satisfies
// it; its methods are nil-safe, so a manager wired without a store simply persists nothing.
type Store interface {
	PutManagerDocument(ctx context.Context, doc store.ManagerDocument) error
	DeleteManagerDocument(ctx context.Context, docKind, docID string) error
	ListManagerDocuments(ctx context.Context, docKind string) ([]store.ManagerDocument, error)
}

// Put write-through-persists value as a JSON document keyed by (kind, id). A nil store is a no-op.
func Put[T any](ctx context.Context, s Store, kind, id, env, tenant string, value T) error {
	if s == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.PutManagerDocument(ctx, store.ManagerDocument{
		DocKind:          kind,
		DocID:            id,
		EnvironmentScope: env,
		TenantID:         tenant,
		DocumentJSON:     string(raw),
	})
}

// Delete removes a document. A nil store is a no-op.
func Delete(ctx context.Context, s Store, kind, id string) error {
	if s == nil {
		return nil
	}
	return s.DeleteManagerDocument(ctx, kind, id)
}

// List reloads all documents of a kind as typed values, skipping any that fail to decode.
func List[T any](ctx context.Context, s Store, kind string) ([]T, error) {
	if s == nil {
		return nil, nil
	}
	docs, err := s.ListManagerDocuments(ctx, kind)
	if err != nil {
		return nil, err
	}
	out := make([]T, 0, len(docs))
	for _, d := range docs {
		var v T
		if json.Unmarshal([]byte(d.DocumentJSON), &v) == nil {
			out = append(out, v)
		}
	}
	return out, nil
}
