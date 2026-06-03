// Package bindings implements Roadmap 58 workspace and capability binding domain
// logic: tenant-owned workspace records, channel/integration-account binding rules,
// capability visibility policy, deterministic precedence resolution, fail-closed
// handling of invalid bindings, and runtime binding evidence construction.
//
// This package is pure domain logic. It owns no persistence and no transport; the
// store, API, chat, and event layers consume the types and resolvers defined here.
// Keeping it dependency-free preserves the boundary the codebase deliberately drew
// between binding configuration and memory, filesystem, provider-auth, and
// connector-routing concerns.
package bindings
