package adapterrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"sync/atomic"
	"time"
)

// DefaultDeadline is applied when a dispatch context carries no deadline (spec clarification
// Q1 / FR-007b). Plan research R3 proposes 30s.
const DefaultDeadline = 30 * time.Second

// CredentialResolver returns scoped, short-lived credential material for a single call
// (Roadmap 37 secret path). It receives the marshaled resource so it can scope to the
// integration/tenant. Returning an error fails the operation closed. Nil material means the
// operation needs no provider credentials (e.g. the reference adapter / fake path).
type CredentialResolver func(ctx context.Context, domain string, resource json.RawMessage) (json.RawMessage, error)

// Client dispatches Backend operations to an out-of-process adapter over the RPC contract.
// It is domain-agnostic: payloads are opaque JSON shaped by the calling domain shim.
type Client struct {
	version         string
	defaultDeadline time.Duration
	t               *transport
	seq             atomic.Int64
	cmd             *exec.Cmd          // non-nil when the client owns a spawned process
	credentials     CredentialResolver // nil when no provider credentials are required
}

// WithCredentials installs a per-call scoped credential resolver and returns the client.
func (c *Client) WithCredentials(r CredentialResolver) *Client {
	c.credentials = r
	return c
}

// WithDefaultDeadline overrides the deadline applied when a dispatch context carries none.
func (c *Client) WithDefaultDeadline(d time.Duration) *Client {
	if d > 0 {
		c.defaultDeadline = d
	}
	return c
}

// NewClient builds a Client over an existing reader/writer pair (used in tests with an
// in-process adapter connected via pipes).
func NewClient(w io.Writer, r io.Reader) *Client {
	return &Client{version: ContractVersion, defaultDeadline: DefaultDeadline, t: newTransport(w, r)}
}

// NewProcessClient spawns the adapter binary and connects to its stdio. The caller owns the
// returned Client and MUST call Close to terminate the process.
func NewProcessClient(ctx context.Context, name string, args ...string) (*Client, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("adapter stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("adapter stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("adapter start: %w", err)
	}
	c := NewClient(stdin, stdout)
	c.cmd = cmd
	return c, nil
}

// Close terminates a spawned adapter process. Safe to call when the client does not own one.
func (c *Client) Close() error {
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	_ = c.cmd.Process.Kill()
	return c.cmd.Wait()
}

func (c *Client) nextRequestID() string {
	return "req-" + strconv.FormatInt(c.seq.Add(1), 10)
}

// Ready performs the contract-version readiness handshake. It returns ErrContractMismatch
// when the adapter advertises a version that is not an exact match for the daemon's (FR-008),
// so the supervisor can refuse to mark the adapter ready before any operation is attempted.
func (c *Client) Ready(ctx context.Context) error {
	return c.Dispatch(ctx, "capability", "Ready", nil, nil, nil)
}

// Dispatch sends one operation: it marshals resource and payload, resolves per-call scoped
// credentials (failing closed on resolver error), validates the contract version and
// correlation, maps adapter failures to *AdapterError, and unmarshals an ok payload into out.
// resource and out may be nil; out is nil for operations with no return value.
func (c *Client) Dispatch(ctx context.Context, domain, operation string, resource, payload, out any) error {
	// Always bound the call daemon-side. If the caller supplied no deadline, apply the client
	// default so a hung adapter can never block indefinitely (FR-007b); advertising deadlineMs
	// alone is not enough — the daemon must enforce it.
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.defaultDeadline)
		defer cancel()
	}

	rawResource, err := marshalRaw(resource)
	if err != nil {
		return fmt.Errorf("marshal %s/%s resource: %w", domain, operation, err)
	}
	rawPayload, err := marshalRaw(payload)
	if err != nil {
		return fmt.Errorf("marshal %s/%s payload: %w", domain, operation, err)
	}

	var credential json.RawMessage
	if c.credentials != nil {
		credential, err = c.credentials(ctx, domain, rawResource)
		if err != nil {
			return fmt.Errorf("resolve %s/%s credentials: %w", domain, operation, err)
		}
	}

	req := Request{
		RequestID:       c.nextRequestID(),
		ContractVersion: c.version,
		Domain:          domain,
		Operation:       operation,
		DeadlineMs:      c.deadlineMs(ctx),
		Resource:        rawResource,
		Credential:      credential,
		Payload:         rawPayload,
	}

	resp, err := c.t.roundTrip(ctx, req)
	if err != nil {
		return err
	}
	if resp.ContractVersion != c.version {
		return ErrContractMismatch
	}
	if resp.RequestID != req.RequestID {
		return ErrCorrelation
	}
	if resp.Status == StatusFailure {
		return &AdapterError{Kind: resp.FailureKind, Detail: diagnosticDetail(resp.Diagnostic)}
	}
	if out != nil && len(resp.Payload) > 0 {
		if err := json.Unmarshal(resp.Payload, out); err != nil {
			// An undecodable ok payload is an unconfirmed outcome for writes (FR-007a).
			return errors.Join(ErrMalformedResponse, err)
		}
	}
	return nil
}

func (c *Client) deadlineMs(ctx context.Context) int64 {
	if dl, ok := ctx.Deadline(); ok {
		remaining := time.Until(dl)
		if remaining < 0 {
			remaining = 0
		}
		return remaining.Milliseconds()
	}
	return c.defaultDeadline.Milliseconds()
}

func marshalRaw(v any) (json.RawMessage, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

func diagnosticDetail(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var d struct {
		Detail  string `json:"detail"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &d); err == nil {
		if d.Detail != "" {
			return d.Detail
		}
		if d.Message != "" {
			return d.Message
		}
	}
	return string(raw)
}
