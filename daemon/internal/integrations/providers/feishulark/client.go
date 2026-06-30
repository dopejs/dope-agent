package feishulark

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterprovider"
	"github.com/dopejs/dope-agent/daemon/internal/integrations/adapterrpc"
)

// DefaultBaseURL is the Feishu Open Platform base. Lark international uses open.larksuite.com;
// it is overridable at wiring time so the same adapter serves either host (and httptest in CI).
const DefaultBaseURL = "https://open.feishu.cn"

type faultKind string

const (
	faultAuth        faultKind = "auth"
	faultScope       faultKind = "scope"
	faultRateLimited faultKind = "rate_limited"
	faultUnavailable faultKind = "unavailable"
	faultInternal    faultKind = "internal"
)

// providerFault is a confirmed provider failure carrying a stable, redacted token. It maps onto
// the adapter contract's failure kinds and the daemon's diagnostics reason vocabulary. It never
// carries credential/token material.
type providerFault struct {
	kind    faultKind
	code    string // stable redacted token, e.g. token_expired, scope_not_granted
	message string // redacted message (no secrets)
}

func (f *providerFault) Error() string {
	if f.message != "" {
		return f.message
	}
	return f.code
}

// toAdapterFault converts to the serve-harness Fault the daemon classifies.
func (f *providerFault) toAdapterFault() *adapterprovider.Fault {
	return &adapterprovider.Fault{Kind: adapterFailureKind(f.kind), Code: f.code, Message: f.message}
}

func adapterFailureKind(k faultKind) adapterrpc.FailureKind {
	switch k {
	case faultAuth:
		return adapterrpc.FailureAuth
	case faultScope:
		return adapterrpc.FailureScope
	case faultRateLimited:
		return adapterrpc.FailureRateLimited
	case faultUnavailable:
		return adapterrpc.FailureUnavailable
	default:
		return adapterrpc.FailureInternal
	}
}

// Client is a thin Feishu Open Platform calendar HTTP client with an injectable base URL and
// *http.Client so it is exercisable against synthetic/recorded responses in CI.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient builds a client. A nil http.Client uses a bounded default; an empty baseURL uses
// DefaultBaseURL.
func NewClient(baseURL string, hc *http.Client) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultBaseURL
	}
	if hc == nil {
		hc = &http.Client{Timeout: 25 * time.Second}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: hc}
}

// envelope is the standard Feishu response wrapper.
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// call performs one request and decodes data into out. write marks side-effecting calls so an
// unconfirmed outcome (mid-response transport break) is reported as ambiguous-commit (FR-008).
func (c *Client) call(ctx context.Context, method, path, token string, body any, out any, write bool) *providerFault {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return &providerFault{kind: faultInternal, code: "request_encode_failed", message: "request body encode failed"}
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return &providerFault{kind: faultInternal, code: "request_build_failed", message: "request build failed"}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.http.Do(req)
	if err != nil {
		// Connection failure. For a write whose request may have reached the provider, the
		// outcome is unconfirmed -> ambiguous-commit; for a read it is a retry-safe unavailable.
		if write && !errors.Is(err, context.Canceled) {
			return ambiguousFault("provider connection broke before acknowledgement")
		}
		return &providerFault{kind: faultUnavailable, code: "provider_unavailable", message: "provider connection failed"}
	}
	defer resp.Body.Close()

	raw, rerr := io.ReadAll(resp.Body)
	if rerr != nil {
		if write {
			return ambiguousFault("provider response truncated after acknowledgement")
		}
		return &providerFault{kind: faultUnavailable, code: "provider_unavailable", message: "provider response read failed"}
	}

	if fault := httpStatusFault(resp.StatusCode, write); fault != nil {
		return fault
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		if write {
			return ambiguousFault("provider acknowledgement unparseable")
		}
		return &providerFault{kind: faultUnavailable, code: "provider_unavailable", message: "provider response unparseable"}
	}
	if env.Code != 0 {
		return feishuCodeFault(env.Code, env.Msg)
	}
	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			if write {
				return ambiguousFault("provider acknowledgement payload unparseable")
			}
			return &providerFault{kind: faultUnavailable, code: "provider_unavailable", message: "provider data unparseable"}
		}
	}
	return nil
}

// ambiguousFault signals an unconfirmed write outcome conveyed to the daemon over the contract's
// ambiguous-commit channel (see adapterprovider.ErrAmbiguous).
func ambiguousFault(message string) *providerFault {
	return &providerFault{kind: faultInternal, code: ambiguousCode, message: message}
}

// ambiguousCode marks a providerFault as an unconfirmed write outcome.
const ambiguousCode = "__ambiguous_commit__"

func (f *providerFault) isAmbiguous() bool { return f != nil && f.code == ambiguousCode }

// httpStatusFault maps transport-level HTTP status to a fault before the Feishu envelope is
// considered. 2xx returns nil so the envelope code governs.
func httpStatusFault(status int, write bool) *providerFault {
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == http.StatusUnauthorized:
		return &providerFault{kind: faultAuth, code: "token_expired", message: "provider rejected credentials"}
	case status == http.StatusForbidden:
		return &providerFault{kind: faultScope, code: "scope_not_granted", message: "provider denied permission"}
	case status == http.StatusTooManyRequests:
		return &providerFault{kind: faultRateLimited, code: "rate_limited", message: "provider rate limited"}
	case status >= 500:
		if write {
			return ambiguousFault("provider returned a server error after a write was submitted")
		}
		return &providerFault{kind: faultUnavailable, code: "service_unavailable", message: "provider service unavailable"}
	default:
		return &providerFault{kind: faultUnavailable, code: "provider_unavailable", message: fmt.Sprintf("provider returned status %d", status)}
	}
}

// feishuCodeFault maps a non-zero Feishu envelope code to a stable, redacted token. The numeric
// code is preserved (it is non-secret and recognized by the daemon's feishu_lark classifier);
// the provider msg is NOT forwarded to avoid leaking sensitive context.
func feishuCodeFault(code int, _ string) *providerFault {
	switch code {
	case 99991663:
		return &providerFault{kind: faultAuth, code: "tenant_access_token_invalid approval", message: "tenant authorization pending"}
	case 99991664, 99991665:
		return &providerFault{kind: faultAuth, code: "app_access_token_invalid", message: "app authorization missing"}
	case 99991668, 99991661:
		return &providerFault{kind: faultAuth, code: "user_access_token_invalid", message: "user authorization invalid"}
	case 99991669:
		return &providerFault{kind: faultScope, code: "scope_not_granted", message: "required scope not granted"}
	case 99991677:
		return &providerFault{kind: faultAuth, code: "token_expired", message: "access token expired"}
	case 1062502, 429:
		return &providerFault{kind: faultRateLimited, code: "rate_limited", message: "provider rate limited"}
	default:
		return &providerFault{kind: faultUnavailable, code: fmt.Sprintf("provider_error_%d", code), message: "provider returned an error"}
	}
}
