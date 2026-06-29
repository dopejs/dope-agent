package adapterrpc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// captureAdapter records the credential each request carried (via a buffered channel to
// avoid a data race) and replies OK.
func captureAdapter(t *testing.T, creds chan<- string) *Client {
	return inlineAdapter(t, func(req Request) Response {
		creds <- string(req.Credential)
		return okResponse(req, `{}`)
	})
}

func TestCredentialsInjectedPerCallScopedToIntegration(t *testing.T) {
	creds := make(chan string, 2)
	c := captureAdapter(t, creds).WithCredentials(ScopedResolver(
		func(ctx context.Context, integrationID string) (json.RawMessage, error) {
			return json.RawMessage(`"secret-for-` + integrationID + `"`), nil
		}))

	for _, id := range []string{"int-1", "int-2"} {
		if err := c.Dispatch(context.Background(), "calendar", "ProjectAccount",
			map[string]any{"integrationId": id}, nil, nil); err != nil {
			t.Fatalf("dispatch %s: %v", id, err)
		}
	}
	got := []string{<-creds, <-creds}
	if got[0] != `"secret-for-int-1"` || got[1] != `"secret-for-int-2"` {
		t.Fatalf("credentials not scoped per call: %v", got)
	}
}

func TestCredentialResolverFailsClosed(t *testing.T) {
	creds := make(chan string, 1)
	c := captureAdapter(t, creds).WithCredentials(ScopedResolver(
		func(ctx context.Context, integrationID string) (json.RawMessage, error) {
			return nil, errors.New("secret unavailable")
		}))

	err := c.Dispatch(context.Background(), "calendar", "CreateEvent",
		map[string]any{"integrationId": "int-1"}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("want fail-closed credential error, got %v", err)
	}
	select {
	case got := <-creds:
		t.Fatalf("operation dispatched despite credential failure (adapter saw %q)", got)
	default:
	}
}
