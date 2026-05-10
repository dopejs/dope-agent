// Package matrix implements the hosted-safe Matrix channel connector boundary.
//
// Phase 52 supports tenant-provided Matrix bot accounts on tenant-selected
// homeservers, unencrypted text direct messages, selected unencrypted rooms with
// bot mention or configured command gates, durable homeserver/conversation/event
// dedupe, final-only foreground replies, connector-backed delivery evidence, and
// redacted diagnostics. It does not operate Matrix homeservers, provision Matrix
// accounts, support encrypted rooms, or implement WhatsApp fallback behavior.
package matrix
