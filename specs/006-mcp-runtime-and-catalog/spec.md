# Feature Specification: Complete MCP Runtime And Catalog

**Feature Branch**: `006-mcp-runtime-and-catalog`  
**Created**: 2026-04-20  
**Status**: Draft  
**Input**: User description: "开始做 harness 的下一步工作：完整的 MCP 实现；参考 OpenClaw 和 HermesAgent 的 MCP 能力与默认可安装 MCP，并在测试环境提供对应能力"

## Clarifications

### Session 2026-04-20

- Q: 当前仓库里已经完成到什么程度？ → A: 已完成 MCP server registry、sandbox-backed lifecycle、credential isolation、tool exposure policy，但尚未完成 end-to-end MCP tool invocation 和默认 installable catalog。
- Q: 这一轮“完整 MCP”以什么为边界？ → A: 以 daemon 内的 MCP client product surface 为边界：可安装、可检查、可启动、可授权、可调用、可审计，并在测试环境提供一组默认 installable MCP 定义。
- Q: 这一轮是否需要照搬外部产品的全部能力？ → A: 不要求完全复制外部产品，但需要吸收 OpenClaw 与 HermesAgent 的核心 MCP 用户面：持久化 server definitions、至少一种远程 transport、tool invocation、以及可直接安装的 starter catalog。
- Q: 默认 installable MCP 需要如何进入测试环境？ → A: 测试环境默认提供 curated MCP catalog 和安装入口；需要外部密钥或额外主机依赖的项可以预置为 installable 模板，但必须如实显示 unavailable reason，不得伪装为可立即使用。
- Q: 第一种远程 MCP transport 选什么？ → A: 选 `streamable-http` 作为第一种远程 MCP transport。
- Q: 首批内置 starter MCP 范围要多大？ → A: 首批做更大 catalog：`filesystem`、`Context7`、`GitHub`、`Postgres`、`Slack` 等。
- Q: catalog entry 通过什么方式安装？ → A: 两种都支持，daemon API 和 repo 脚本并存。
- Q: MCP tool invocation 应该接到哪里？ → A: MCP tool invocation 直接并入现有 runtime/tool-call 主平面。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - MCP tools are actually callable through the daemon (Priority: P1)

As an operator or runtime surface, I need daemon-managed MCP tools to be invocable through
the existing runtime and tool-call boundary so MCP is a usable harness subsystem rather
than only a server registry and policy shell.

**Why this priority**: The current repository can register, start, and inspect MCP
servers, but it still stops short of end-to-end MCP tool execution. Without this, MCP is
not complete from the product perspective.

**Independent Test**: Register a representative MCP server, allowlist a discovered tool
for a runtime surface, invoke that tool through the daemon, and verify the result,
approval behavior, sandbox provenance, and failure classification all stay operator
visible.

**Acceptance Scenarios**:

1. **Given** an MCP server is enabled, healthy, and exposes an allowlisted tool,
   **When** a runtime surface invokes that tool through the daemon, **Then** the daemon
   executes the MCP call through the owning server session inside the existing
   `/v1/runs/.../tool-calls` plane and records the tool call, authorization decision,
   server identity, and result.
2. **Given** an MCP tool is approval-gated, **When** a runtime surface invokes it without
   prior approval, **Then** the daemon returns an approval-required outcome tied to the
   MCP tool identity rather than silently bypassing the policy layer.
3. **Given** an MCP server is unhealthy, disabled, or stale, **When** a runtime surface
   attempts to invoke one of its tools, **Then** the daemon returns an explicit blocked or
   unavailable outcome instead of hanging or misclassifying the failure as a normal local
   tool runtime error.

---

### User Story 2 - Operators can install curated MCP servers into the test environment (Priority: P1)

As an operator validating the harness, I need a curated installable MCP catalog with
starter server definitions so I can bring up useful MCP integrations in the test
environment without reconstructing JSON config by hand.

**Why this priority**: OpenClaw and HermesAgent both reduce MCP friction by shipping saved
definitions and documented starters. Without an installable catalog, our MCP subsystem is
technically present but operationally incomplete.

**Independent Test**: Inspect the catalog, install representative starter definitions into
`DOPE_ENV=test`, and verify each installed server appears as a first-class MCP resource
with truthful prerequisite, credential, and availability state.

**Acceptance Scenarios**:

1. **Given** the daemon is running in the test environment, **When** an operator lists the
   installable MCP catalog, **Then** the daemon returns a curated set of installable
   server definitions with transport kind, prerequisite summary, secret requirements, and
   availability hints.
2. **Given** an installable MCP definition targets the test environment and its local
   prerequisites are satisfied, **When** the operator installs it, **Then** the daemon
   persists a first-class MCP server resource and surfaces the resulting config through
   the existing MCP inspection routes.
3. **Given** an installable MCP definition requires secrets or external dependencies that
   are not currently satisfied, **When** the operator installs or inspects it, **Then**
   the daemon reports it as blocked or unavailable with an explicit reason instead of
   implying it is ready.
4. **Given** the bundled catalog includes immediate-use entries and credential-backed
   templates, **When** the operator inspects the catalog, **Then** each entry clearly
   distinguishes whether it is immediately usable in `DOPE_ENV=test` or requires optional
   credentials or external infrastructure.
5. **Given** an operator prefers either daemon-managed install or repo-local automation,
   **When** they install a catalog entry, **Then** both the daemon API path and the
   repo-supported script path converge on the same installed MCP server resource model and
   audit truth.

---

### User Story 3 - MCP transport support covers both local stdio and at least one remote mode (Priority: P2)

As an operator comparing our MCP support with OpenClaw and HermesAgent, I need the daemon
to support both local stdio MCP servers and at least one remote MCP transport so the
catalog can include both local starter servers and remote documentation-style services.

**Why this priority**: Our current MCP client only supports stdio. That blocks parity with
common real-world MCP usage and prevents us from shipping representative starters such as
remote docs/context services.

**Independent Test**: Register one stdio MCP server and one remote MCP server, then
confirm discovery, lifecycle truth, tool invocation, and failure classification remain
consistent across both transport families.

**Acceptance Scenarios**:

1. **Given** an MCP server uses stdio transport, **When** the daemon starts and invokes
   it, **Then** existing sandbox-backed lifecycle semantics remain intact.
2. **Given** an MCP server uses the supported remote transport for this slice, **When**
   the daemon inspects or invokes it, **Then** transport-specific health and failure
   state stay explicit without creating a second unmanaged MCP plane.
3. **Given** a catalog item depends on a transport the daemon does not yet support,
   **When** the operator inspects that item, **Then** it is marked unavailable or
   unsupported rather than silently installed into a broken state.

---

### User Story 4 - MCP starter catalog remains truthful and auditable (Priority: P3)

As an operator or future engineer, I need the bundled MCP catalog and the daemon-visible
MCP state to remain aligned so the project can support curated integrations without hidden
test-only behavior or undocumented defaults.

**Why this priority**: Shipping bundled MCP definitions introduces a new contract surface:
catalog truth, install behavior, and operator docs must agree or the test environment will
drift from the codebase.

**Independent Test**: Follow the documented MCP install-and-verify workflow for bundled
servers, then confirm docs, API responses, and event history all describe the same catalog
entries, prerequisites, install results, and invocation behavior.

**Acceptance Scenarios**:

1. **Given** a bundled MCP definition is advertised to operators, **When** the operator
   inspects docs or API state, **Then** the same definition name, transport, prerequisites,
   and secret expectations are visible in both places.
2. **Given** a bundled MCP server is installed and later disabled, removed, or becomes
   unhealthy, **When** the operator inspects the system, **Then** the catalog entry and
   installed server state remain distinguishable and auditable.

### Edge Cases

- What happens when a catalog item is installable in `DOPE_ENV=test` but its required
  package manager or binary is absent on the host?
- How does the daemon represent a remote MCP server whose endpoint is configured but
  currently unreachable?
- What happens when a bundled MCP definition requires secrets that are intentionally absent
  from the test environment?
- How does the system handle a tool schema change on a running MCP server after install?
- What happens when an MCP tool invocation is requested while the owning server is being
  restarted or backing off?
- How does the daemon prevent a stale catalog definition from overwriting an operator-edited
  installed MCP server without an explicit update action?
- What happens when a remote MCP transport is supported for invocation but is not eligible
  for sandbox-backed local process lifecycle?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST let daemon-managed MCP tools be invoked through the existing
  runtime and tool-call plane rather than limiting MCP to registration and inspection.
- **FR-001a**: This slice MUST NOT introduce a second standalone MCP invocation plane as
  the primary execution path; MCP tool invocation MUST reuse the existing runtime/tool-call
  surface as the canonical operator-visible execution story.
- **FR-002**: MCP tool invocation MUST preserve operator-visible provenance linking the
  tool call to the owning MCP server, transport, authorization result, and resulting MCP
  session state.
- **FR-003**: The system MUST enforce MCP tool exposure and approval policy at invocation
  time and MUST NOT allow direct MCP tool use outside the daemon-managed authorization
  path.
- **FR-004**: The system MUST distinguish MCP tool invocation failure classes including
  blocked, approval-required, approval-rejected, server-unhealthy, transport-failed,
  timeout, and tool-level remote error.
- **FR-005**: The system MUST support `stdio` MCP servers end-to-end for discovery and
  invocation without regressing the existing sandbox-backed lifecycle model.
- **FR-006**: The system MUST support at least one remote MCP transport in addition to
  `stdio`, and the first remote transport in this slice MUST be `streamable-http`.
- **FR-006a**: The transport choice for each MCP server MUST remain operator-visible.
- **FR-007**: The system MUST provide a curated installable MCP catalog that can be listed
  and inspected through operator-visible daemon surfaces.
- **FR-008**: Catalog entries MUST include stable identity, display name, transport kind,
  prerequisite summary, secret requirements, environment eligibility, and installation
  instructions or arguments sufficient for daemon-managed install.
- **FR-009**: Operators MUST be able to install a catalog entry into the test environment
  through daemon-managed or repo-supported workflow without hand-authoring the full MCP
  server definition JSON.
- **FR-009a**: This slice MUST support both daemon API installation and repo-supported
  script installation for bundled MCP catalog entries, and both paths MUST converge on the
  same installed MCP server resource shape.
- **FR-010**: Installed catalog entries MUST become first-class MCP server resources using
  the same registry, lifecycle, secret, and exposure semantics as manually created MCP
  servers.
- **FR-011**: The bundled MCP catalog for this slice MUST include a curated set derived
  from the OpenClaw and HermesAgent MCP user surface, including local filesystem-oriented
  servers and at least one remote docs/context-oriented server.
- **FR-011a**: The initial bundled catalog for this slice MUST include at least
  `filesystem`, `Context7`, `GitHub`, `Postgres`, and `Slack` starter entries, while
  preserving truthful unavailable or blocked status for entries whose credentials or host
  prerequisites are not yet satisfied.
- **FR-012**: Any bundled MCP entry that depends on unavailable binaries, endpoints, or
  secrets MUST be marked with an explicit unavailable or blocked reason rather than being
  implied ready.
- **FR-013**: Test and production environments MUST remain separated for installed MCP
  entries, secret resolution, and prerequisite checks.
- **FR-014**: MCP catalog and install behavior MUST remain additive to current MCP
  registry surfaces so existing MCP API consumers do not require a breaking migration.
- **FR-015**: Operator-visible configuration, event, and history surfaces MUST continue to
  redact secret values and secret-derived material for installed and invoked MCP servers.
- **FR-016**: The system MUST document which bundled MCP entries are intended for immediate
  test-environment use, which require optional credentials, and which remain templates for
  later activation.
- **FR-017**: Verification for this slice MUST include at least one real end-to-end MCP
  tool invocation from an installed catalog entry in the test environment.
- **FR-018**: Verification for this slice MUST include both a prerequisite-satisfied path
  and a truthful unavailable path for bundled MCP entries.

### Key Entities *(include if feature involves data)*

- **Installable MCP Definition**: A curated MCP server template with stable id, display
  metadata, transport kind, command or endpoint details, prerequisite summary, secret
  requirements, environment eligibility, and installation defaults.
- **Installed MCP Server**: A first-class daemon-managed MCP server resource created from
  a catalog entry or manual definition and governed by the standard registry and lifecycle
  surfaces.
- **MCP Tool Invocation Record**: The runtime tool-call and audit record representing a
  daemon-mediated call into an MCP tool, including authorization result, server identity,
  transport, and outcome.
- **MCP Transport Session**: The daemon-owned client connection to an MCP server, whether
  stdio or remote, used for discovery and invocation.

## Compatibility & Operational Impact *(mandatory)*

- **Compatibility Impact**: Adds MCP invocation routes or runtime integration, installable
  MCP catalog surfaces, additional MCP transport schema, and new contract/event projection
  for installed catalog entries and MCP tool invocation.
- **Migration / Rollback**: Rollout is additive. Rollback removes MCP invocation and
  catalog surfaces while preserving existing MCP registry and lifecycle resources.
- **Verification Strategy**: Validate local package-level tests for `daemon/internal/mcp`,
  `daemon/internal/api`, and any runtime integration paths; run `make daemon-contract-test`;
  verify at least one bundled MCP entry installs and invokes successfully in
  `DOPE_ENV=test`; verify unavailable-path classification for unmet prerequisites.
- **Observability Impact**: MCP catalog inspection, install events, invocation audit
  records, and docs must expose transport, prerequisite, availability, and authorization
  truth.
- **Environment & Secrets**: Default validation stays in `DOPE_ENV=test`; bundled entries
  with optional external credentials must remain installable without leaking secrets and
  must clearly surface blocked state when credentials are absent.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can list the bundled MCP catalog, install a starter entry, and
  inspect its resulting MCP server resource in under 5 minutes using committed docs and
  daemon-visible routes only.
- **SC-002**: At least one bundled MCP entry can be installed and invoked end-to-end in
  the test environment through the daemon-managed MCP and tool-call plane.
- **SC-003**: 100% of validated bundled MCP prerequisite failures surface an explicit
  unavailable or blocked reason rather than generic install or runtime failure.
- **SC-004**: 100% of validated MCP tool invocations preserve owning server identity,
  authorization state, and transport information in operator-visible records.
- **SC-005**: The bundled MCP catalog covers at least one local stdio server family and at
  least one remote docs/context server family inspired by the OpenClaw and HermesAgent MCP
  starter experience, and includes a broader starter set spanning local, credential-backed,
  and service-oriented integrations.

## Assumptions

- The current MCP registry, lifecycle, secret-scope, and tool-exposure implementation from
  `specs/003-mcp-execution-plane` remains the base and is not re-architected in this slice.
- “Complete MCP” for the harness means MCP is usable end-to-end for installation,
  lifecycle, authorization, and invocation through daemon-managed surfaces, not only
  inspectable as server state.
- The bundled starter catalog may include entries that are immediately usable in test and
  entries that are installable templates but intentionally blocked until secrets or remote
  endpoints are configured.
- The initial starter catalog is intentionally broader than the minimum matrix and includes
  both immediate-use and template entries such as `Context7`, `filesystem`, `GitHub`,
  `Postgres`, and `Slack`, with truthful blocked or unavailable status when a bundled
  default still requires a local override or credential.
- The first remote MCP transport added in this slice can be whichever mode best matches
  the selected starter catalog; for this slice, that transport is `streamable-http`.
- OpenClaw and HermesAgent are used as external UX references for saved MCP definitions,
  starter catalog shape, and transport coverage; exact byte-for-byte parity is out of
  scope.
