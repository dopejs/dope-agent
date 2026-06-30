package feishulark

import (
	"encoding/json"
	"strings"
)

// scopedToken is the per-call credential envelope the daemon resolves (Roadmap 37 secret path)
// and passes to the adapter. It carries only short-lived access material and granted scopes;
// the adapter never persists or logs it.
type scopedToken struct {
	AccessToken   string   `json:"accessToken"`
	TokenType     string   `json:"tokenType,omitempty"` // "user" (default) or "tenant"
	GrantedScopes []string `json:"grantedScopes,omitempty"`
}

// parseToken decodes the credential envelope, failing closed when material is absent so a
// missing/empty credential never reaches the provider as an anonymous call (FR-012).
func parseToken(raw json.RawMessage) (scopedToken, *providerFault) {
	if len(raw) == 0 {
		return scopedToken{}, &providerFault{kind: faultAuth, code: "access_token_missing", message: "no credential material resolved for integration"}
	}
	var tok scopedToken
	if err := json.Unmarshal(raw, &tok); err != nil {
		return scopedToken{}, &providerFault{kind: faultAuth, code: "access_token_missing", message: "credential envelope unreadable"}
	}
	if strings.TrimSpace(tok.AccessToken) == "" {
		return scopedToken{}, &providerFault{kind: faultAuth, code: "access_token_missing", message: "credential envelope carried no access token"}
	}
	return tok, nil
}

// hasScope reports whether the granted scope set includes want. An empty granted set is treated
// as "unknown / not restricted" so read paths are not blocked before the provider rules; write
// scope is enforced provider-side and surfaced as scope_not_granted on rejection.
func (t scopedToken) hasScope(want string) bool {
	if len(t.GrantedScopes) == 0 {
		return true
	}
	for _, s := range t.GrantedScopes {
		if strings.EqualFold(strings.TrimSpace(s), want) {
			return true
		}
	}
	return false
}
