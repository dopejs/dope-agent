package evidence

import (
	"regexp"
	"strings"
)

// sensitiveKeys are summary keys whose values are always redacted.
var sensitiveKeys = map[string]bool{
	"token": true, "accesstoken": true, "access_token": true, "refreshtoken": true,
	"secret": true, "password": true, "authorization": true, "apikey": true, "api_key": true,
	"credential": true, "oauth": true, "clientsecret": true, "client_secret": true, "signingsecret": true,
}

// secretMarker matches obvious raw credential material that must never appear in a bundle. If a
// value still matches after redaction, the bundle fails closed.
var secretMarker = regexp.MustCompile(`(?i)(bearer\s+[a-z0-9._\-]{8,}|sk-[a-z0-9]{8,}|xox[baprs]-[a-z0-9-]{8,}|-----BEGIN [A-Z ]+PRIVATE KEY-----|eyJ[a-zA-Z0-9_\-]{10,}\.)`)

const redactedPlaceholder = "[redacted]"

// redactSections redacts sensitive summary values and validates that no raw secret material
// remains. It fails closed (returns false) if any residual secret marker is detected.
func redactSections(sections []Section) ([]Section, bool) {
	out := make([]Section, 0, len(sections))
	ok := true
	for _, section := range sections {
		redacted := Section{Kind: section.Kind, ResourceRefs: section.ResourceRefs, Links: section.Links}
		if len(section.Summary) > 0 {
			redacted.Summary = make(map[string]string, len(section.Summary))
			for key, value := range section.Summary {
				if sensitiveKeys[strings.ToLower(strings.TrimSpace(key))] {
					redacted.Summary[key] = redactedPlaceholder
					continue
				}
				if secretMarker.MatchString(value) {
					// A non-sensitive-keyed value carrying raw secret material cannot be safely
					// redacted in place — fail the whole bundle closed (FR redaction-fail-closed).
					ok = false
					redacted.Summary[key] = redactedPlaceholder
					continue
				}
				redacted.Summary[key] = value
			}
		}
		// Links must not carry secret material either.
		for _, link := range section.Links {
			if secretMarker.MatchString(link) {
				ok = false
			}
		}
		out = append(out, redacted)
	}
	return out, ok
}
