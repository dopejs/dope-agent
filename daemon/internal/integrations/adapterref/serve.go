// Package adapterref is the reference integration adapter skeleton (Roadmap 59). It speaks
// the capability RPC contract over stdio but contains NO real provider: read operations
// return empty deterministic payloads and mutations return empty objects. It supports seeded
// failure modes so the conformance harness and failure-isolation tests can exercise the
// daemon's classification. Real providers (Google Calendar, Gmail) replace it in Roadmap 60/63.
package adapterref

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterrpc"
)

// FailMode seeds a deterministic failure for testing/conformance.
type FailMode string

const (
	FailNone      FailMode = ""
	FailAuth      FailMode = "auth"      // respond StatusFailure auth (confirmed non-commit)
	FailMalformed FailMode = "malformed" // emit a non-contract frame (undecodable response)
	FailHang      FailMode = "hang"      // sleep past any deadline before responding
	FailCrash     FailMode = "crash"     // stop serving (process/stream dies) without responding
)

// Options configures the reference adapter.
type Options struct {
	FailMode FailMode
	// FailOperations, when non-empty, restricts FailMode to those operation names (e.g.
	// {"CreateEvent": true}) so reads such as ProjectAccount still succeed. Empty means the
	// FailMode applies to every operation.
	FailOperations map[string]bool
	HangFor        time.Duration // used with FailHang; defaults to 2s
	ContractVer    string        // override advertised contract version (for mismatch tests)
}

func (o Options) failApplies(operation string) bool {
	if o.FailMode == FailNone {
		return false
	}
	if len(o.FailOperations) == 0 {
		return true
	}
	return o.FailOperations[operation]
}

// arrayOps return a JSON array; all other operations return a JSON object.
var arrayOps = map[string]bool{
	"ListEvents":         true,
	"ListThreads":        true,
	"ListDrafts":         true,
	"ResolveAttachments": true,
}

// Serve runs the stdio loop with default (no-failure) options.
func Serve(in io.Reader, out io.Writer) error {
	return ServeWithOptions(in, out, Options{})
}

// ServeWithOptions runs the stdio request/response loop honoring the seeded failure mode.
func ServeWithOptions(in io.Reader, out io.Writer, opts Options) error {
	br := bufio.NewReader(in)
	for {
		req, err := adapterrpc.ReadRequest(br)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		apply := opts.failApplies(req.Operation)
		if apply {
			switch opts.FailMode {
			case FailCrash:
				// Simulate the adapter dying mid-operation: stop without responding.
				return nil
			case FailHang:
				d := opts.HangFor
				if d == 0 {
					d = 2 * time.Second
				}
				time.Sleep(d)
			case FailMalformed:
				if _, werr := out.Write([]byte("this-is-not-json\n")); werr != nil {
					return werr
				}
				continue
			}
		}

		if werr := adapterrpc.WriteMessage(out, handle(req, opts, apply)); werr != nil {
			return werr
		}
	}
}

// Handle produces a deterministic, contract-valid OK response for a request. Exposed for
// tests that drive a single request.
func Handle(req adapterrpc.Request) adapterrpc.Response {
	return handle(req, Options{}, false)
}

func handle(req adapterrpc.Request, opts Options, apply bool) adapterrpc.Response {
	version := adapterrpc.ContractVersion
	if opts.ContractVer != "" {
		version = opts.ContractVer
	}
	if apply && opts.FailMode == FailAuth {
		return adapterrpc.Response{
			RequestID:       req.RequestID,
			ContractVersion: version,
			Status:          adapterrpc.StatusFailure,
			FailureKind:     adapterrpc.FailureAuth,
		}
	}
	var payload json.RawMessage
	if arrayOps[req.Operation] {
		payload = json.RawMessage("[]")
	} else {
		payload = json.RawMessage("{}")
	}
	return adapterrpc.Response{
		RequestID:       req.RequestID,
		ContractVersion: version,
		Status:          adapterrpc.StatusOK,
		Payload:         payload,
	}
}

// NewPipeClient runs the reference adapter in-process over pipes and returns a connected
// client plus a stop function.
func NewPipeClient() (*adapterrpc.Client, func()) {
	return NewPipeClientWithOptions(Options{})
}

// NewPipeClientWithOptions is NewPipeClient with seeded failure/version options.
func NewPipeClientWithOptions(opts Options) (*adapterrpc.Client, func()) {
	adapterReader, daemonWriter := io.Pipe() // daemon -> adapter
	daemonReader, adapterWriter := io.Pipe() // adapter -> daemon
	go func() {
		_ = ServeWithOptions(adapterReader, adapterWriter, opts)
		// Closing the write side mirrors a real process exit (stdout closes -> daemon read
		// observes EOF -> ErrUnreachable), so a crash mode does not hang the caller.
		_ = adapterWriter.Close()
	}()
	client := adapterrpc.NewClient(daemonWriter, daemonReader)
	stop := func() {
		_ = daemonWriter.Close()
		_ = adapterWriter.Close()
	}
	return client, stop
}
