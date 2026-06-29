package adapterrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
)

// US3 / FR-015: concurrent operations on one shared adapter must be isolated per call — no
// cross-bleed of credentials, content, or results. The echo adapter returns each request's
// credential as its payload; every concurrent caller must get back its own.
func TestConcurrentCallsAreIsolatedPerCall(t *testing.T) {
	c := inlineAdapter(t, func(req Request) Response {
		return Response{RequestID: req.RequestID, ContractVersion: ContractVersion, Status: StatusOK, Payload: req.Credential}
	}).WithCredentials(ScopedResolver(func(ctx context.Context, integrationID string) (json.RawMessage, error) {
		return json.RawMessage(`"` + integrationID + `"`), nil
	}))

	const n = 25
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("int-%d", i)
			var got string
			if err := c.Dispatch(context.Background(), "calendar", "ProjectAccount",
				map[string]any{"integrationId": id}, nil, &got); err != nil {
				errs <- fmt.Errorf("%s: %w", id, err)
				return
			}
			if got != id {
				errs <- fmt.Errorf("cross-bleed: caller %s received %q", id, got)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
