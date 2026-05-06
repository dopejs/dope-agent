# Operator-Managed Skill And Capability Catalog

Status: proposed

Authority: This document is the authoritative upstream spec for Roadmap 67, the
operator-managed skill and capability catalog slice before agent-managed skills.

Primary source documents:
- `docs/harness/skill-registry-and-prompt-support.md`
- `docs/specs/007-mcp-catalog-management-and-distribution.md`
- `docs/specs/043-workspace-and-capability-binding.md`
- `docs/product/feature-phasing.md`

## Background

OpenClaw and HermesAgent emphasize skills and extensibility. DopeAgent already has MCP,
skills, sandbox, and capability primitives, but public parity requires an operator-managed
catalog with enablement, permission, version, rollback, and hosted-safe install policy.
Agent-generated skills remain later work after memory.

## Goal

Provide an operator-managed catalog for skills and capabilities that can be enabled,
disabled, permissioned, versioned, inspected, and rolled back safely.

## Fixed Decisions

- This roadmap does not allow the agent to generate or promote its own skills.
- Hosted install policy must be explicit and fail closed.
- Catalog items must declare requirements and trust tier.
- Capability visibility must integrate with profile/workspace bindings.

## Dependencies On Completed Phases

- Roadmap 22: MCP Catalog Management And Distribution
- Roadmap 58: Workspace And Capability Binding
- Roadmap 37: Hosted Secrets, Integrations, And Connector Isolation

## In Scope

- catalog item resource for skills, MCP servers, and supervised capabilities
- version, source, trust tier, requirement declaration, and enabled state
- install, disable, rollback, and inspect actions
- tenant permission and capability visibility integration
- hosted-safe policy and unsupported install reasons
- operator shell management views

## Out Of Scope

- community marketplace
- agent-authored skills
- automatic skill promotion
- memory-based procedural learning

## Operator Or User Problems To Solve

- Users need to see which skills/capabilities are enabled.
- Operators need safe rollback and requirement visibility before enabling risky tools.

## User Stories

- As an operator, I can enable a vetted capability for a tenant profile.
- As an operator, I can roll back a capability version.
- As a user, I can see why a requested skill is unavailable.

## Functional Requirements

- Catalog items MUST declare source, version, trust tier, requirements, and permissions.
- Enablement MUST be tenant-scoped and auditable.
- Policy MUST block unmet requirements before install or execution.
- Rollback MUST restore a prior enabled version or disable safely.
- Runtime evidence MUST identify catalog item versions that influenced execution.

## Compatibility And Operational Notes

Existing skill and MCP registries should feed catalog projections where possible rather
than being replaced wholesale.

## Verification Expectations

- API/store/schema tests for catalog lifecycle.
- Policy tests for unmet requirements and permission denial.
- Runtime evidence tests for active catalog item version.
- Web tests for enable, disable, rollback, and inspect.

## Definition Of Done

- DopeAgent has skill/capability catalog parity for operator-managed extensions without
  entering agent-managed skill generation.

## Recommended `/speckit-specify` Input

`$speckit-specify 结合 docs/specs/052-operator-managed-skill-and-capability-catalog.md 完成 phase 67 的工作`
