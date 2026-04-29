// Package livevalidation owns Roadmap 40 live replay safety policy.
//
// The package keeps live validation separate from non-live evaluation replay. It
// coordinates support-matrix readiness, permission and quota gates, fresh
// approvals, kill switches, side-effect ledger evidence, retry/abort decisions,
// ambiguous commit handling, reconciliation, retention, and outcome comparison.
package livevalidation
