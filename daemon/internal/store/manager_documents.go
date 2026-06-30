package store

import (
	"context"
	"time"
)

// ManagerDocument is a generic JSON-document row backing the Roadmap 65-71 managers (triage,
// routine, webhook, catalog, execprofile, evidence). Each manager serializes its resources as
// documents keyed by (DocKind, DocID) and reloads them on startup.
type ManagerDocument struct {
	DocKind          string
	DocID            string
	EnvironmentScope string
	TenantID         string
	DocumentJSON     string
	UpdatedAt        time.Time
}

// PutManagerDocument inserts or replaces a manager document.
func (s *SQLiteStore) PutManagerDocument(ctx context.Context, doc ManagerDocument) error {
	if s == nil || s.db == nil {
		return nil
	}
	updatedAt := doc.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO manager_documents (doc_kind, doc_id, environment_scope, tenant_id, document_json, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(doc_kind, doc_id) DO UPDATE SET
			environment_scope = excluded.environment_scope,
			tenant_id = excluded.tenant_id,
			document_json = excluded.document_json,
			updated_at = excluded.updated_at
	`, doc.DocKind, doc.DocID, doc.EnvironmentScope, doc.TenantID, doc.DocumentJSON, updatedAt.UTC().Format(time.RFC3339Nano))
	return err
}

// DeleteManagerDocument removes a manager document.
func (s *SQLiteStore) DeleteManagerDocument(ctx context.Context, docKind, docID string) error {
	if s == nil || s.db == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM manager_documents WHERE doc_kind = ? AND doc_id = ?`, docKind, docID)
	return err
}

// ListManagerDocuments returns all documents for a kind, ordered by document id for determinism.
func (s *SQLiteStore) ListManagerDocuments(ctx context.Context, docKind string) ([]ManagerDocument, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT doc_kind, doc_id, environment_scope, tenant_id, document_json, updated_at
		FROM manager_documents WHERE doc_kind = ? ORDER BY doc_id
	`, docKind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ManagerDocument
	for rows.Next() {
		var doc ManagerDocument
		var updatedAt string
		if err := rows.Scan(&doc.DocKind, &doc.DocID, &doc.EnvironmentScope, &doc.TenantID, &doc.DocumentJSON, &updatedAt); err != nil {
			return nil, err
		}
		if t, perr := time.Parse(time.RFC3339Nano, updatedAt); perr == nil {
			doc.UpdatedAt = t
		}
		out = append(out, doc)
	}
	return out, rows.Err()
}
