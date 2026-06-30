// Package adapterprovider runs the adapter side of the capability RPC contract (Roadmap 59)
// against a real provider Handler, replacing the empty-payload reference skeleton for real
// providers (Roadmap 60/63). It owns the stdio loop, the contract-version handshake, per-call
// deadline derivation, and failure/diagnostic shaping. It performs provider request/response
// mapping only: it never records ledger, idempotency, or side-effect state, and it MUST NOT
// emit credential or token material in any payload or diagnostic.
package adapterprovider

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterrpc"
)

// Handler maps one capability RPC operation to a result payload. Returning a *Fault yields a
// StatusFailure response carrying the typed failure kind and a redacted diagnostic; any other
// non-nil error yields a StatusFailure internal. resource/credential/payload are the raw
// request envelopes. The returned payload and any *Fault MUST be free of secret material.
type Handler interface {
	Handle(ctx context.Context, op Operation) (json.RawMessage, error)
}

// Operation is the decoded operation request handed to a Handler.
type Operation struct {
	Domain     string
	Operation  string
	Resource   json.RawMessage
	Credential json.RawMessage
	Payload    json.RawMessage
}

// ErrAmbiguous signals an unconfirmed write outcome (FR-008): the provider acknowledgement
// could not be confirmed (success-then-disconnect, truncated, or unparseable ack). The wire
// contract has no dedicated ambiguous failure kind, so the harness conveys it to the daemon
// over the contract's undecodable-response channel, which the daemon classifies as
// ambiguous-commit. A Handler returns this (possibly wrapped) for an unconfirmed write.
var ErrAmbiguous = errors.New("provider write outcome ambiguous")

// Fault is a confirmed provider failure. Kind classifies it for daemon diagnostics and
// live-validation; Code is a stable, redacted token (e.g. "token_expired", "scope_not_granted")
// surfaced as the diagnostic detail and used by the daemon's reason classifier.
type Fault struct {
	Kind    adapterrpc.FailureKind
	Code    string
	Message string
}

func (f *Fault) Error() string {
	if f.Message != "" {
		return f.Message
	}
	return f.Code
}

// maxHandlerDeadline caps the per-call deadline the adapter derives from the request so a
// missing/oversized deadlineMs cannot let a provider call run unbounded.
const maxHandlerDeadline = 2 * time.Minute

// Serve runs the request/response loop against h until EOF.
func Serve(in io.Reader, out io.Writer, h Handler) error {
	br := bufio.NewReader(in)
	for {
		req, err := adapterrpc.ReadRequest(br)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		resp, ambiguous := dispatch(req, h)
		if ambiguous {
			// Convey an unconfirmed write outcome over the contract's undecodable-response
			// channel: an intentionally non-contract frame the daemon reads as ambiguous-commit.
			if _, werr := out.Write([]byte("ambiguous-commit-unconfirmed\n")); werr != nil {
				return werr
			}
			continue
		}
		if werr := adapterrpc.WriteMessage(out, resp); werr != nil {
			return werr
		}
	}
}

func dispatch(req adapterrpc.Request, h Handler) (adapterrpc.Response, bool) {
	// The contract-version readiness handshake is answered locally; it carries no provider work.
	if req.Domain == "capability" && req.Operation == "Ready" {
		return okResponse(req, nil), false
	}

	ctx := context.Background()
	if req.DeadlineMs > 0 {
		d := time.Duration(req.DeadlineMs) * time.Millisecond
		if d > maxHandlerDeadline {
			d = maxHandlerDeadline
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}

	payload, err := h.Handle(ctx, Operation{
		Domain:     req.Domain,
		Operation:  req.Operation,
		Resource:   req.Resource,
		Credential: req.Credential,
		Payload:    req.Payload,
	})
	if err != nil {
		if errors.Is(err, ErrAmbiguous) {
			return adapterrpc.Response{}, true
		}
		return failureResponse(req, err), false
	}
	return okResponse(req, payload), false
}

func okResponse(req adapterrpc.Request, payload json.RawMessage) adapterrpc.Response {
	return adapterrpc.Response{
		RequestID:       req.RequestID,
		ContractVersion: adapterrpc.ContractVersion,
		Status:          adapterrpc.StatusOK,
		Payload:         payload,
	}
}

func failureResponse(req adapterrpc.Request, err error) adapterrpc.Response {
	kind := adapterrpc.FailureInternal
	code := "provider_internal_error"
	message := ""
	var fault *Fault
	if errors.As(err, &fault) {
		if fault.Kind != "" {
			kind = fault.Kind
		}
		if fault.Code != "" {
			code = fault.Code
		}
		message = fault.Message
	}
	diag, _ := json.Marshal(map[string]string{"detail": code, "message": message})
	return adapterrpc.Response{
		RequestID:       req.RequestID,
		ContractVersion: adapterrpc.ContractVersion,
		Status:          adapterrpc.StatusFailure,
		FailureKind:     kind,
		Diagnostic:      diag,
	}
}
