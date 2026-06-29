package adapterrpc

import (
	"bufio"
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

// transport carries one request/response round trip over a newline-delimited stdio stream.
// Calls are serialized: a single stdio stream cannot interleave frames, and serialization
// also guarantees per-call isolation across concurrent callers (FR-015) at the wire.
//
// Once a round trip times out or hits a stream error, the transport is "broken": a leaked
// reader may still be consuming t.r, and the frame stream may be desynchronized, so every
// subsequent call fails fast with ErrUnreachable instead of spawning a competing reader on
// the same bufio.Reader. Recovery requires a fresh client (supervisor restart / respawn).
type transport struct {
	mu     sync.Mutex
	w      io.Writer
	r      *bufio.Reader
	broken atomic.Bool
}

func newTransport(w io.Writer, r io.Reader) *transport {
	return &transport{w: w, r: bufio.NewReader(r)}
}

// roundTrip writes the request and waits for the correlated response, honoring the context
// deadline. On deadline expiry it returns ErrTimeout (hang/timeout classification, FR-007b)
// and poisons the transport; the caller treats that as an unconfirmed outcome.
func (t *transport) roundTrip(ctx context.Context, req Request) (Response, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.broken.Load() {
		return Response{}, ErrUnreachable
	}

	if err := WriteMessage(t.w, req); err != nil {
		t.broken.Store(true)
		return Response{}, ErrUnreachable
	}

	type result struct {
		resp Response
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		resp, err := ReadResponse(t.r)
		ch <- result{resp: resp, err: err}
	}()

	select {
	case <-ctx.Done():
		// Deadline expiry: the outcome is unconfirmed (hang/timeout). Poison the transport so
		// the leaked reader is never raced by a subsequent call; it drains when the adapter
		// eventually responds or the stream closes (e.g. on process kill).
		t.broken.Store(true)
		return Response{}, errors.Join(ErrTimeout, ctx.Err())
	case res := <-ch:
		if res.err != nil {
			t.broken.Store(true)
			if errors.Is(res.err, io.EOF) || errors.Is(res.err, io.ErrClosedPipe) {
				return Response{}, ErrUnreachable
			}
			return Response{}, errors.Join(ErrMalformedResponse, res.err)
		}
		return res.resp, nil
	}
}
