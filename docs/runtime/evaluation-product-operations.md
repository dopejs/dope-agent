# Evaluation Product Operations

Roadmap 41 product evaluation actions are additive and should default to the
test environment. Operators can disable new product actions while preserving
historical evidence for audit.

## Disable New Product Work

1. Disable discovery scheduling and reject new discovery starts.
2. Disable product fixture creation, editing, review, and suppression changes.
3. Disable campaign starts, cancellation side effects, and result publication.
4. Disable dashboard projection generation.
5. Leave read-only product evidence available to authorized users.

## Retention And Deletion

Retention/deletion applies to product-managed evaluation resources only:

- discovered candidates
- candidate evidence
- product-managed fixture payloads and selectability
- campaign result detail payloads
- dashboard projections
- tool-call inspection payloads

The current Roadmap 41 implementation persists tenant-scoped retention state for
candidate evidence, discovered candidates, product fixtures, campaigns,
dashboard projections, and tool-call inspections. Store retention can be
applied in tests and operator tooling; the public `POST
/v1/evaluation/retention/apply` route remains disabled for product mutation
until an operator rollout flag is introduced.

Retention must not delete repo-managed fixture files, runtime truth, Roadmap 33
replay attempts, or Roadmap 40 live-validation ledger records. When product
payloads are removed, tombstones and audit records keep enough metadata to
explain campaign history and future selection decisions.

## Evidence Expectations

Operator-visible evidence should show tenant id, actor, target resource,
outcome, reason code, bounds or policy reference where relevant, and redacted
evidence references. Redaction failures must fail closed and produce audit
evidence without exposing sensitive material.

## Verification

Before Roadmap 41 is complete, run targeted Go tests, full daemon tests,
contract tests, client tests/build, daemon smoke in `~/.dope-test`, and the
Roadmap 39 soak rerun with Roadmap 40 live validation and Roadmap 41 product
workflows included.
