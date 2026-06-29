package adapterrpc

import (
	"context"
	"encoding/json"
)

// IntegrationCredentialFetcher returns scoped, short-lived credential material for one
// integration. At wiring time it is backed by the Roadmap 37 secret path; it MUST return
// fresh material per call and MUST NOT be cached by the adapter.
type IntegrationCredentialFetcher func(ctx context.Context, integrationID string) (json.RawMessage, error)

// ScopedResolver builds a per-call CredentialResolver that derives the integration id from
// the resource envelope and fetches fresh credential material for each call. Because it
// fetches per call and threads nothing through the shared adapter process, concurrent calls
// for different tenants cannot share credential state (FR-006, FR-015).
func ScopedResolver(fetch IntegrationCredentialFetcher) CredentialResolver {
	return func(ctx context.Context, domain string, resource json.RawMessage) (json.RawMessage, error) {
		id := integrationIDFromResource(resource)
		if fetch == nil || id == "" {
			return nil, nil
		}
		return fetch(ctx, id)
	}
}

func integrationIDFromResource(resource json.RawMessage) string {
	if len(resource) == 0 {
		return ""
	}
	var r struct {
		IntegrationID string `json:"integrationId"`
	}
	_ = json.Unmarshal(resource, &r)
	return r.IntegrationID
}
