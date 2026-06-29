// Package adapterrpc implements the daemon side of the external integration adapter plane
// (Roadmap 59). It defines the schema-aligned RPC envelopes, a newline-delimited JSON codec,
// a stdio transport, and a Client that domain Backend shims (calendar, mail) use to dispatch
// operations to an out-of-process adapter.
//
// The adapter performs provider request/response mapping only. The daemon retains the
// operation ledger, idempotency, side-effect evidence, artifacts, and persistence; nothing
// in this package records that state.
package adapterrpc

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ContractVersion is the daemon's integration-adapter contract version. The daemon requires
// an exact match with the adapter (FR-008, spec clarification Q3).
const ContractVersion = "1"

// Status is the outcome of an adapter operation.
type Status string

const (
	StatusOK      Status = "ok"
	StatusFailure Status = "failure"
)

// FailureKind classifies an adapter failure so the daemon can map it to existing integration
// diagnostics and live-validation classification (FR-007).
type FailureKind string

const (
	FailureAuth        FailureKind = "auth"
	FailureScope       FailureKind = "scope"
	FailureRateLimited FailureKind = "rate_limited"
	FailureUnavailable FailureKind = "unavailable"
	FailureMalformed   FailureKind = "malformed"
	FailureInternal    FailureKind = "internal"
)

// Request is the daemon -> adapter envelope for one Backend operation.
// Mirrors schemas/capability/integration-adapter/request.json.
type Request struct {
	RequestID       string          `json:"requestId"`
	ContractVersion string          `json:"contractVersion"`
	Domain          string          `json:"domain"`
	Operation       string          `json:"operation"`
	DeadlineMs      int64           `json:"deadlineMs"`
	Resource        json.RawMessage `json:"resource,omitempty"`
	Credential      json.RawMessage `json:"credential,omitempty"`
	Payload         json.RawMessage `json:"payload,omitempty"`
}

// Response is the adapter -> daemon envelope.
// Mirrors schemas/capability/integration-adapter/response.json. It MUST NOT carry ledger,
// idempotency, or side-effect-evidence state (FR-003).
type Response struct {
	RequestID       string          `json:"requestId"`
	ContractVersion string          `json:"contractVersion"`
	Status          Status          `json:"status"`
	FailureKind     FailureKind     `json:"failureKind,omitempty"`
	Payload         json.RawMessage `json:"payload,omitempty"`
	Diagnostic      json.RawMessage `json:"diagnostic,omitempty"`
}

// ErrContractMismatch is returned when the adapter advertises a contract version that is not
// an exact match for the daemon's (FR-008).
var ErrContractMismatch = errors.New("integration adapter contract version mismatch")

// ErrCorrelation is returned when a response's requestId does not match the request.
var ErrCorrelation = errors.New("integration adapter response correlation mismatch")

// The following errors describe outcomes where a side-effecting operation's result cannot be
// confirmed (deadline expiry, transport break, or an undecodable response). For a write,
// these are ambiguous-commit conditions (FR-007a): the daemon must not assume committed or
// failed. A clean StatusFailure response (see AdapterError) is, by contrast, a confirmed
// non-commit and is NOT ambiguous.
var (
	ErrTimeout           = errors.New("integration adapter operation timed out")
	ErrUnreachable       = errors.New("integration adapter unreachable")
	ErrMalformedResponse = errors.New("integration adapter response malformed")
)

// IsAmbiguous reports whether err represents an unconfirmed outcome (FR-007a).
func IsAmbiguous(err error) bool {
	return errors.Is(err, ErrTimeout) || errors.Is(err, ErrUnreachable) || errors.Is(err, ErrMalformedResponse)
}

// AdapterError carries a typed adapter failure back to the domain shim, which maps it onto
// existing integration diagnostics / live-validation classification.
type AdapterError struct {
	Kind   FailureKind
	Detail string
}

func (e *AdapterError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("integration adapter failure (%s): %s", e.Kind, e.Detail)
	}
	return fmt.Sprintf("integration adapter failure (%s)", e.Kind)
}
