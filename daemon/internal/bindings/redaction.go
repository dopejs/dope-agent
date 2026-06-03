package bindings

import (
	"strings"
	"unicode/utf8"
)

// maxSafeLabelLen bounds any user-facing label surfaced from binding state.
const maxSafeLabelLen = 160

// SafeLabel returns a bounded, secret-free version of a label suitable for binding
// inspection, runtime evidence, events, and audit output (FR-028, SC-014). Unsafe or
// empty input collapses to a neutral placeholder so nothing sensitive leaks.
func SafeLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "(unnamed)"
	}
	if containsUnsafe(value) {
		return "(redacted)"
	}
	// Strip control characters and collapse whitespace.
	var b strings.Builder
	lastSpace := false
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			continue
		}
		if r == ' ' || r == '\t' {
			if lastSpace {
				continue
			}
			lastSpace = true
			b.WriteRune(' ')
			continue
		}
		lastSpace = false
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "(unnamed)"
	}
	if len(out) > maxSafeLabelLen {
		return truncateBytes(out, maxSafeLabelLen)
	}
	return out
}

// truncateBytes returns a valid UTF-8 string whose total byte length (including the
// "…" marker) does not exceed limit.
func truncateBytes(value string, limit int) string {
	const marker = "…"
	budget := limit - len(marker)
	if budget <= 0 {
		return marker
	}
	trimmed := value[:budget]
	for len(trimmed) > 0 && !utf8.ValidString(trimmed) {
		trimmed = trimmed[:len(trimmed)-1]
	}
	return trimmed + marker
}

// SafeReason returns a bounded, secret-free reason code/string for denial evidence.
func SafeReason(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unspecified"
	}
	if containsUnsafe(value) {
		return "redacted"
	}
	if len(value) > maxSafeLabelLen {
		return truncateBytes(value, maxSafeLabelLen)
	}
	return value
}

// RedactionStatusFor reports whether redaction was applied to a label. A label that had
// to be scrubbed reports RedactionRedacted; otherwise RedactionNotRequired.
func RedactionStatusFor(original string) RedactionStatus {
	if SafeLabel(original) != strings.TrimSpace(original) {
		return RedactionRedacted
	}
	return RedactionNotRequired
}
