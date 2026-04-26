// Package inventory loads the schema inventory checked in under
// specs/020-tenant-scoped-data-migration/contracts/schema-inventory.md and
// verifies its completeness against the live SQLite schema and registered
// event sources.
//
// Roadmap 35 requires every persisted table and every event-bearing record
// source to be classified as `tenant_owned`, `global`, or `derived`. The
// completeness test in this package fails when the live schema and the
// inventory drift apart, so a newly added table or event source cannot
// silently bypass tenant classification.
package inventory
