export type EnvironmentScope = "test" | "prod";

export type ChatQueryInput = {
  provider?: string;
  model?: string;
  skills?: string[];
  query: string;
  timeoutMs?: number;
  maxRetries?: number;
};

export type ChatQueryResponse = {
  dispatchId: string;
  provider: string;
  model: string;
  skills: string[];
  query: string;
  status: "queued" | "running" | "completed" | "partial_failed" | "failed" | "cancelled";
  partial: boolean;
  reply: string;
  finishReason?: string;
  usage: {
    inputTokens: number;
    outputTokens: number;
    totalTokens: number;
  };
  errorCode?: string;
  error?: string;
};

export type ChatQueryStreamStarted = {
  dispatchId: string;
  provider: string;
  model: string;
  skills: string[];
  query: string;
};

export type ChatQueryStreamDelta = {
  dispatchId: string;
  delta: string;
  reply: string;
};

export type StreamHandlers = {
  onStarted?: (payload: ChatQueryStreamStarted) => void;
  onDelta?: (payload: ChatQueryStreamDelta) => void;
  onCompleted?: (payload: ChatQueryResponse) => void;
  onFailed?: (payload: ChatQueryResponse) => void;
  onCancelled?: (payload: ChatQueryResponse) => void;
};

export type OperatorReadinessItem = {
  itemId: string;
  itemKind: string;
  resourceId?: string;
  displayName: string;
  status: "ready" | "blocked" | "degraded" | "missing_configuration" | "optional";
  healthState?: string;
  reason?: string;
  requiredOperatorAction?: string;
  requiredForSelectedAction: boolean;
  detailRoute?: string;
  environmentScope: EnvironmentScope;
  updatedAt: string;
};

export type OperatorFirstUsefulAction = {
  actionId: string;
  actionKind: string;
  displayName: string;
  recommended: boolean;
  available: boolean;
  blockingItemIds?: string[];
  summary?: string;
  invokeRoute: string;
  resultRoute?: string;
};

export type OperatorOnboardingResponse = {
  environmentScope: EnvironmentScope;
  status: "blocked" | "ready_for_action" | "completed";
  currentStepId?: string;
  completedStepIds?: string[];
  blockingItemIds: string[];
  optionalFollowUpItemIds: string[];
  recommendedActionId?: string;
  readinessItems: OperatorReadinessItem[];
  firstUsefulActions: OperatorFirstUsefulAction[];
  lastEvaluatedAt: string;
};

export type OperatorResourceRef = {
  kind: string;
  id: string;
  route?: string;
};

export type OperatorActivityRecord = {
  activityId: string;
  sourceKind: string;
  sourceId: string;
  title: string;
  status: string;
  summary: string;
  attentionLevel: "info" | "warning" | "critical";
  occurredAt: string;
  detailRoute?: string;
  relatedResourceRefs?: OperatorResourceRef[];
  environmentScope: EnvironmentScope;
};

export type OperatorActivityListResponse = {
  environmentScope: EnvironmentScope;
  items: OperatorActivityRecord[];
  generatedAt: string;
};

export type OperatorDiagnosticFinding = {
  findingId: string;
  sourceKind: string;
  sourceId: string;
  plane: "readiness" | "approval" | "execution" | "delivery";
  severity: "warning" | "critical";
  status: string;
  reason: string;
  recommendedAction?: string;
  detailRoute?: string;
  relatedResourceRefs?: OperatorResourceRef[];
  environmentScope: EnvironmentScope;
  capturedAt: string;
};

export type OperatorDiagnosticListResponse = {
  environmentScope: EnvironmentScope;
  items: OperatorDiagnosticFinding[];
  generatedAt: string;
};

export type OperatorActivityQuery = {
  sourceKind?: string;
  attentionOnly?: boolean;
  limit?: number;
};

export type OperatorDiagnosticsQuery = {
  sourceKind?: string;
  plane?: OperatorDiagnosticFinding["plane"];
  severity?: OperatorDiagnosticFinding["severity"];
};

export type ReplaySourceRef = {
  kind: string;
  id: string;
  route?: string;
};

export type PlaneSummaries = {
  runtime?: string;
  policy?: string;
  integration?: string;
  delivery?: string;
  evidence?: string;
};

export type ReplayCandidateResource = {
  candidateId: string;
  candidateKind: "curated_work" | "fixture";
  displayName: string;
  description?: string;
  sourceKind: "run" | "workflow" | "schedule" | "integration" | "computer_use" | "fixture";
  sourceId: string;
  sourceRefs: ReplaySourceRef[];
  toolClasses?: string[];
  environmentScope: EnvironmentScope;
  readinessStatus: "fully_replayable" | "partially_replayable" | "blocked" | "unreplayable";
  readinessReasons: string[];
  limitations: string[];
  defaultReplayMode: "non_live";
  fixtureId?: string;
  latestAttemptId?: string;
  latestComparisonId?: string;
  expectedComparisonSummary?: PlaneSummaries;
  capturedEvidenceRefs?: ReplaySourceRef[];
  createdAt: string;
  updatedAt: string;
};

export type ReplayCandidateListResponse = {
  environmentScope: EnvironmentScope;
  items: ReplayCandidateResource[];
};

export type ReplayCandidateQuery = {
  candidateKind?: ReplayCandidateResource["candidateKind"];
  sourceKind?: ReplayCandidateResource["sourceKind"];
  readinessStatus?: ReplayCandidateResource["readinessStatus"];
  limit?: number;
};

export type CreateReplayCandidateInput = Omit<ReplayCandidateResource, "createdAt" | "updatedAt" | "latestAttemptId" | "latestComparisonId"> & {
  createdAt?: string;
  updatedAt?: string;
};

export type CreateReplayAttemptInput = {
  mode?: "non_live" | "live_validation";
  changeWindowLabel?: string;
  baselineAttemptId?: string;
  safetyScope?: {
    mode?: "non_live" | "live_validation";
    description?: string;
  };
};

export type ReplayAttemptResource = {
  attemptId: string;
  candidateId: string;
  sourceRefs: ReplaySourceRef[];
  environmentScope: EnvironmentScope;
  mode: "non_live" | "live_validation";
  status: "queued" | "running" | "completed" | "blocked" | "unreplayable" | "failed" | "cancelled";
  safetyScope: {
    mode?: "non_live" | "live_validation";
    description?: string;
  };
  approvalHandling: "blocked" | "evidence_only" | "fresh_approval_required";
  sideEffectHandling: "blocked" | "evidence_only" | "live";
  launchedBy?: string;
  changeWindowLabel?: string;
  baselineAttemptId?: string;
  resultRunId?: string;
  resultWorkflowId?: string;
  evidenceRefs: ReplaySourceRef[];
  blockedReasons: string[];
  runtimeSummary?: string;
  policySummary?: string;
  integrationSummary?: string;
  deliverySummary?: string;
  evidenceSummary?: string;
  startedAt?: string;
  completedAt?: string;
  createdAt: string;
  updatedAt: string;
};

export type ReplayAttemptListResponse = {
  environmentScope: EnvironmentScope;
  items: ReplayAttemptResource[];
};

export type ReplayAttemptQuery = {
  candidateId?: string;
  status?: ReplayAttemptResource["status"];
  limit?: number;
};

export type DriftFindingResource = {
  findingId: string;
  comparisonId: string;
  plane: "runtime" | "policy" | "integration" | "delivery" | "evidence" | "unknown" | "mixed";
  severity: "info" | "warning" | "critical";
  summary: string;
  baselineValue?: string;
  replayValue?: string;
  evidenceRefs?: ReplaySourceRef[];
  recommendedAction?: string;
  createdAt: string;
};

export type CreateReplayComparisonInput = {
  baselineAttemptId?: string;
  baselineRef?: string;
  changeWindowLabel?: string;
};

export type ReplayComparisonResource = {
  comparisonId: string;
  candidateId: string;
  baselineRef: string;
  attemptId: string;
  environmentScope: EnvironmentScope;
  terminalStatus: "matched" | "drifted" | "blocked" | "unreplayable";
  runtimeSummary: string;
  policySummary: string;
  integrationSummary: string;
  deliverySummary: string;
  evidenceSummary: string;
  confidence: "high" | "medium" | "low";
  limitations: string[];
  driftFindings: DriftFindingResource[];
  changeWindowLabel?: string;
  generatedAt: string;
};

export type ReplayComparisonListResponse = {
  environmentScope: EnvironmentScope;
  items: ReplayComparisonResource[];
};

export type ReplayComparisonQuery = {
  candidateId?: string;
  attemptId?: string;
  terminalStatus?: ReplayComparisonResource["terminalStatus"];
  limit?: number;
};

export type ReplayFixtureResource = {
  fixtureId: string;
  displayName: string;
  domainClass: "schedule" | "integration" | "computer_use";
  manifestPath: string;
  sourceRefs: ReplaySourceRef[];
  capturedEvidenceRefs: ReplaySourceRef[];
  assumptions: string[];
  limitations: string[];
  expectedReplayMode: "non_live" | "live_validation";
  expectedComparisonSummary: PlaneSummaries;
  candidateId?: string;
  environmentScope: EnvironmentScope;
  createdAt: string;
  updatedAt: string;
};

export type ReplayFixtureListResponse = {
  environmentScope: EnvironmentScope;
  items: ReplayFixtureResource[];
};

export type ReplayFixtureQuery = {
  domainClass?: ReplayFixtureResource["domainClass"];
  limit?: number;
};

export type LiveValidationAttemptStatus =
  | "queued"
  | "awaiting_approval"
  | "running"
  | "completed"
  | "blocked"
  | "aborted"
  | "failed"
  | "operator_action_needed";

export type LiveValidationLedgerOutcome =
  | "attempted"
  | "skipped"
  | "completed"
  | "failed"
  | "aborted"
  | "denied"
  | "operator_action_needed";

export type LiveValidationSafetyClass =
  | "read_only"
  | "idempotent_mutation"
  | "non_idempotent_mutation"
  | "unsupported";

export type LiveValidationGateDecision = {
  allowed: boolean;
  reasonCode?: string;
  reference?: string;
  checkedAt: string;
};

export type LiveValidationSideEffectScope = {
  scopeId: string;
  validationId: string;
  includedToolClasses?: string[];
  excludedToolClasses?: string[];
  includedActions?: string[];
  excludedActions?: string[];
  approvalMode: "scope_level" | "per_action" | "mixed";
  declaredBy: string;
  declaredAt: string;
};

export type LiveValidationAttemptResource = {
  validationId: string;
  tenantId?: string;
  candidateId: string;
  sourceAttemptId?: string;
  requestedBy: string;
  environmentScope: EnvironmentScope | string;
  requestedScope: LiveValidationSideEffectScope;
  status: LiveValidationAttemptStatus;
  permissionDecision: LiveValidationGateDecision;
  quotaDecision: LiveValidationGateDecision;
  killSwitchDecision: LiveValidationGateDecision;
  approvalSummary: {
    required: number;
    approved: number;
    denied: number;
    expired: number;
    pending: number;
  };
  ledgerSummary: Partial<Record<LiveValidationLedgerOutcome, number>>;
  comparisonId?: string;
  createdAt: string;
  startedAt?: string;
  completedAt?: string;
  updatedAt: string;
};

export type LiveValidationAttemptListResponse = {
  tenantId?: string;
  environmentScope?: EnvironmentScope | string;
  items: LiveValidationAttemptResource[];
};

export type LiveValidationApprovalEvidence = {
  approvalId: string;
  validationId: string;
  tenantId?: string;
  approvalTarget: "scope" | "action";
  toolClass: string;
  safetyClass: LiveValidationSafetyClass;
  actionRef?: string;
  approvedScope?: string;
  status: "pending" | "approved" | "denied" | "expired";
  requestedBy: string;
  resolvedBy?: string;
  requestedAt: string;
  resolvedAt?: string;
};

export type CreateLiveValidationInput = {
  validationId?: string;
  candidateId: string;
  sourceAttemptId?: string;
  candidateToolClasses?: string[];
  requestedScope: LiveValidationSideEffectScope;
  freshApprovals?: LiveValidationApprovalEvidence[];
  clientKey?: string;
  changeWindowLabel?: string;
};

export type CreateLiveValidationResponse = {
  attempt: LiveValidationAttemptResource;
  denials?: Array<{
    gate: "permission" | "quota" | "kill_switch" | "support_matrix" | "approval" | "scope" | string;
    reasonCode: string;
    message: string;
    reference?: string;
  }>;
};

export type LiveValidationAttemptQuery = {
  candidateId?: string;
  status?: LiveValidationAttemptStatus;
  limit?: number;
};

export type LiveValidationLedgerResource = {
  ledgerEntryId: string;
  validationId: string;
  tenantId?: string;
  candidateId: string;
  sourceRef: string;
  toolClass: string;
  safetyClass: LiveValidationSafetyClass;
  actionRef: string;
  approvalId?: string;
  correlationKey?: string;
  downstreamRef?: string;
  outcome: LiveValidationLedgerOutcome;
  reasonCode?: string;
  attemptedAt?: string;
  completedAt?: string;
  updatedAt: string;
  evidenceRefs?: string[];
  retryCount: number;
  ambiguousCommit: boolean;
  reconciliationId?: string;
};

export type LiveValidationLedgerListResponse = {
  validationId: string;
  tenantId?: string;
  items: LiveValidationLedgerResource[];
};

export type LiveValidationSupportMatrixResource = {
  toolClass: string;
  safetyClass: LiveValidationSafetyClass;
  permission?: TenantPermission | string;
  approval: "not_required" | "scope_level" | "per_action" | "unsupported";
  approvalAction?: string;
  idempotency?: string;
  retryPolicy: "automatic_retry" | "manual_retry" | "no_retry";
  ambiguousCommitBehavior?: string;
  compensation: "not_applicable" | "automatic_compensation" | "manual_confirmation" | "unsupported";
  ledgerEvents: LiveValidationLedgerOutcome[];
  testCase: string;
  version: string;
};

export type LiveValidationSupportMatrixResponse = {
  environmentScope?: EnvironmentScope | string;
  version: string;
  items: LiveValidationSupportMatrixResource[];
};

export type LiveValidationComparisonResource = {
  comparisonId: string;
  validationId: string;
  candidateId: string;
  baselineRef: string;
  terminalStatus: "matched" | "drifted" | "blocked" | "unsupported" | "operator_action_needed";
  ledgerSummary: Partial<Record<LiveValidationLedgerOutcome, number>>;
  unsupportedClasses?: string[];
  denials?: string[];
  ambiguousCommits?: string[];
  driftFindings?: string[];
  generatedAt: string;
};

export type ResolveLiveValidationReconciliationInput = {
  resolution: "confirmed_committed" | "confirmed_not_committed" | "compensated" | "accepted_manual_state" | "unsupported_unresolved";
  reason: string;
  evidenceRefs?: string[];
};

export type LiveValidationReconciliationResource = {
  reconciliationId: string;
  ambiguousCommitId: string;
  tenantId?: string;
  resolvedBy: string;
  resolution: ResolveLiveValidationReconciliationInput["resolution"];
  reason: string;
  evidenceRefs?: string[];
  resolvedAt: string;
};

export type LiveValidationRetentionResource = {
  policyId: string;
  tenantId?: string;
  appliesTo: "attempts" | "ledger_entries" | "reconciliation_decisions" | "comparisons" | "all";
  mode: "indefinite" | "explicit";
  retentionPeriod?: string;
  createdByPrincipalId: string;
  createdAt: string;
  expiresAt?: string;
};

export type UpdateLiveValidationKillSwitchInput = {
  scope: "tenant" | "global";
  tenantId?: string;
  enabled: boolean;
  reason: string;
  expiresAt?: string;
};

export type LiveValidationKillSwitchResource = {
  killSwitchId: string;
  scope: "tenant" | "global";
  tenantId?: string;
  enabled: boolean;
  reason: string;
  changedBy: string;
  changedAt: string;
  expiresAt?: string;
};

export type LiveValidationKillSwitchListResponse = {
  tenantId?: string;
  items: LiveValidationKillSwitchResource[];
};

export type ApprovalStatus = "pending" | "approved" | "rejected";

export type ApprovalResource = {
  approvalId: string;
  action: string;
  resourceKind?: string;
  resourceId?: string;
  reason: string;
  requestedBy?: string;
  status: ApprovalStatus;
  createdAt: string;
  updatedAt: string;
  resolvedAt?: string;
  resolution?: string;
  comment?: string;
  integrationBindings?: Array<Record<string, unknown>>;
  sandbox?: Record<string, unknown>;
};

export type ApprovalListResponse = {
  items: ApprovalResource[];
};

export type DecisionResource = {
  decisionId: string;
  action: string;
  resourceKind?: string;
  resourceId?: string;
  outcome: string;
  reason: string;
  approvalId?: string;
  createdAt: string;
  sandbox?: Record<string, unknown>;
};

export type ApprovalDecisionResponse = {
  approval: ApprovalResource;
  decision: DecisionResource;
};

export type ResolveApprovalInput = {
  resolution: "approved" | "rejected";
  comment?: string;
};

export type SessionRouteRequest = {
  kind?: string;
  channel?: string;
  accountId?: string;
  peerId?: string;
  threadId?: string;
};

export type CreateRunInput = {
  sessionId?: string;
  route?: SessionRouteRequest;
  entrypoint: string;
  goal?: string;
  input?: unknown;
};

export type RunResource = {
  runId: string;
  sessionId?: string;
  scheduleId?: string;
  scheduleAttemptId?: string;
  reminderId?: string;
  reminderOccurrenceId?: string;
  entrypoint: string;
  status: string;
  goal: string;
  activeWorkflowId?: string;
  workflowCount?: number;
  latestDeliveryId?: string;
  latestDeliveryStatus?: string;
  latestDeliveryTargetId?: string;
  createdAt: string;
  updatedAt: string;
};

export type DaemonEventScope = {
  sessionId?: string;
  runId?: string;
  workflowId?: string;
  workflowStepId?: string;
  scheduleId?: string;
  scheduleAttemptId?: string;
  stepId?: string;
  computerUseSessionId?: string;
  computerUseActionId?: string;
  connectorId?: string;
  capabilityId?: string;
};

export type DaemonEvent = {
  eventId: string;
  sequence: number;
  category: string;
  name: string;
  occurredAt: string;
  scope: DaemonEventScope;
  resource: {
    kind: string;
    id: string;
  };
  payload: Record<string, unknown>;
};

export type EventStreamQuery = {
  category?: string;
  runId?: string;
  sessionId?: string;
  scheduleId?: string;
  scheduleAttemptId?: string;
  resourceKind?: string;
  cursor?: number;
};

export type EventStreamHandlers = {
  onEvent?: (event: DaemonEvent) => void;
  onError?: (error: Error) => void;
};

export type EventStreamSubscription = {
  close: () => void;
  completed: Promise<void>;
};

export type TenantRole = "owner" | "admin" | "operator" | "viewer";
export type TenantKind = "personal" | "organization";
export type TenantLifecycleStatus =
  | "invited"
  | "pending"
  | "active"
  | "disabled"
  | "removed"
  | "rejected"
  | "revoked"
  | "expired"
  | "accepted"
  | "rotated";

export type TenantPermission =
  | "tenant.manage"
  | "secrets.manage"
  | "credentials.inspect"
  | "integrations.manage"
  | "connectors.manage"
  | "mcp.manage"
  | "runs.execute"
  | "approvals.resolve"
  | "live_validation.execute"
  | "live_validation.reconcile"
  | "evaluation.manage"
  | "billing.view"
  | "billing.manage"
  | "read_only.inspect";

export type TenantResource = {
  tenantId: string;
  tenantKind: TenantKind;
  displayName: string;
  status: TenantLifecycleStatus;
  createdAt: string;
  updatedAt: string;
  createdByPrincipalId?: string;
  defaultOwnerPrincipalId?: string;
  callerMembershipRole?: TenantRole;
  callerMembershipStatus?: TenantLifecycleStatus;
  callerPermissions?: TenantPermission[];
  defaultForCurrentToken?: boolean;
  defaultForCurrentPrincipal?: boolean;
};

export type PrincipalResource = {
  principalId: string;
  principalKind: "local_operator" | "user" | "service_client";
  displayName: string;
  status: TenantLifecycleStatus;
  defaultTenantId: string;
  createdAt: string;
  updatedAt: string;
  disabledAt?: string;
  removedAt?: string;
};

export type MembershipResource = {
  membershipId: string;
  tenantId: string;
  principalId: string;
  role: TenantRole;
  status: TenantLifecycleStatus;
  invitationId?: string;
  createdAt: string;
  updatedAt: string;
  acceptedAt?: string;
  removedAt?: string;
};

export type TokenTenantGrantResource = {
  grantId: string;
  tokenId: string;
  tenantId: string;
  isDefault: boolean;
  status: TenantLifecycleStatus;
  createdAt: string;
  updatedAt: string;
  revokedAt?: string;
  grantedByPrincipalId?: string;
};

export type TenantContextResource = {
  principalId: string;
  tokenId: string;
  tenantId: string;
  tenantSource: string;
  membershipId?: string;
  role?: TenantRole;
  permissions: TenantPermission[];
  resolvedAt: string;
};

export type TenantDenialResource = {
  error: string;
  errorCode: string;
  requestId?: string;
};

export type AuthMeResponse = {
  token: Record<string, unknown>;
  principal: PrincipalResource;
  defaultTenant: TenantResource;
  currentTenant: TenantResource;
  allowedTenants: TenantResource[];
  tokenGrants: TokenTenantGrantResource[];
  permissions: TenantPermission[];
  tenantContext: TenantContextResource;
};

export type TenantListResponse = {
  items: TenantResource[];
};

export type TenantDetailResponse = {
  tenant: TenantResource;
  tenantContext: TenantContextResource;
};

export type MembershipListResponse = {
  items: MembershipResource[];
};

export type UpdateMembershipRoleInput = {
  role: TenantRole;
};

export type TenantRequestOptions = {
  tenantId?: string;
};

export type BillingEnforcementMode = "enforced" | "unlimited" | "not_measurable";

export type BillingPlanResource = {
  planId: string;
  tenantId?: string;
  planKey: string;
  status: "active" | "scheduled" | "disabled" | "superseded" | string;
  enforcementMode: BillingEnforcementMode;
  effectiveAt: string;
  supersededAt?: string;
  assignedByPrincipalId?: string;
  assignmentReason?: string;
  document?: Record<string, unknown>;
};

export type BillingQuotaResource = {
  tenantId?: string;
  planKey: string;
  category: string;
  unit: string;
  periodStart: string;
  periodEnd: string;
  periodAnchor?: string;
  limit: number;
  consumedAmount: number;
  reservedAmount: number;
  adjustedAmount: number;
  carryoverApplied: number;
  remainingAmount: number;
  enforcementMode: BillingEnforcementMode;
  denialReasonCode?: string;
  overLimit?: boolean;
};

export type BillingDenialResource = {
  denialId: string;
  tenantId?: string;
  category?: string;
  quotaPeriodId?: string;
  operationKey: string;
  reasonCode: string;
  requestedAmount: number;
  remainingAmount: number;
  guardedEntryPoint: string;
  createdAt: string;
};

export type BillingManualAdjustmentResource = {
  adjustmentId: string;
  tenantId?: string;
  category: string;
  quotaPeriodId: string;
  amountDelta: number;
  reason: string;
  createdByPrincipalId?: string;
  createdAt: string;
};

export type BillingUsageResponse = {
  tenantId?: string;
  planKey: string;
  enforcementMode: BillingEnforcementMode;
  quotas: BillingQuotaResource[];
  manualAdjustments?: BillingManualAdjustmentResource[];
  denials?: BillingDenialResource[];
};

export type BillingPlanAssignmentInput = {
  planKey: string;
  enforcementMode: BillingEnforcementMode;
  reason: string;
};

export type BillingQuotaOverrideInput = {
  category: string;
  limit?: number;
  reason: string;
};

export type BillingManualAdjustmentInput = {
  category: string;
  quotaPeriodId: string;
  amountDelta: number;
  reason: string;
};

export type BillingReservationResource = {
  reservationId: string;
  tenantId?: string;
  category: string;
  quotaPeriodId: string;
  operationKey: string;
  amountReserved: number;
  amountCommitted?: number;
  amountRefunded?: number;
  status: string;
  reservationPoint?: string;
  commitPoint?: string;
  refundPoint?: string;
  createdAt: string;
  updatedAt: string;
  expiresAt?: string;
  recoveryReason?: string;
};

export type BillingReservationResolutionInput = {
  outcome: "committed" | "released" | "refunded" | "operator_action_needed";
  reason: string;
  amount?: number;
};

export type BillingQuotaDenialPayload = {
  code: "quota_denied";
  reasonCode: string;
  tenantId?: string;
  category?: string;
  operationKey?: string;
  requestedAmount?: number;
  remainingAmount?: number;
  periodStart?: string;
  periodEnd?: string;
};

export type SecretStatus = "active" | "disabled" | "pending_remediation";
export type SecretResolutionStatus = "resolved" | "unavailable" | "denied" | "not_applicable";

export type RedactedSecretSummary = {
  secretRef?: string;
  secretVersionId?: string;
  resolution?: SecretResolutionStatus;
  status?: SecretStatus;
  disabledReason?: string;
  redactionRule?: string;
};

export type TenantSecretResource = {
  secretId: string;
  tenantId: string;
  secretRef: string;
  displayName?: string;
  status: SecretStatus;
  activeVersionId?: string;
  disabledReason?: string;
  remediationReason?: string;
  createdAt: string;
  updatedAt: string;
  rotatedAt?: string;
  disabledAt?: string;
  document?: Record<string, unknown>;
  secretRefs?: RedactedSecretSummary[];
};

export type TenantSecretListResponse = {
  items: TenantSecretResource[];
};

export type CreateTenantSecretInput = {
  secretRef: string;
  displayName?: string;
  value: string;
  document?: Record<string, unknown>;
};

export type UpdateTenantSecretInput = {
  displayName?: string;
  document?: Record<string, unknown>;
};

export type RotateTenantSecretInput = {
  value: string;
};

export type DisableTenantSecretInput = {
  disabledReason: string;
};

export type TenantSecretResponse = {
  secret: TenantSecretResource;
};

export type DopeClientOptions = {
  baseURL: string;
  accessToken?: string;
  fetchImpl?: typeof fetch;
  defaultTenantId?: string;
};

export class DopeClientError extends Error {
  readonly status: number;
  readonly code?: string;
  readonly tenantDenied?: boolean;
  readonly denial?: TenantDenialResource;
  readonly quotaDenial?: BillingQuotaDenialPayload;

  constructor(message: string, options: { status: number; code?: string; tenantDenied?: boolean; denial?: TenantDenialResource; quotaDenial?: BillingQuotaDenialPayload }) {
    super(message);
    this.name = "DopeClientError";
    this.status = options.status;
    this.code = options.code;
    this.tenantDenied = options.tenantDenied;
    this.denial = options.denial;
    this.quotaDenial = options.quotaDenial;
  }
}

export class DopeClient {
  private readonly baseURL: string;
  private readonly accessToken?: string;
  private readonly fetchImpl: typeof fetch;
  private readonly defaultTenantId?: string;

  constructor(options: DopeClientOptions) {
    this.baseURL = trimBaseURL(options.baseURL);
    this.accessToken = options.accessToken?.trim() || undefined;
    this.fetchImpl = options.fetchImpl ?? globalThis.fetch.bind(globalThis);
    this.defaultTenantId = options.defaultTenantId?.trim() || undefined;
  }

  async queryChat(input: ChatQueryInput, tenantOptions?: TenantRequestOptions): Promise<ChatQueryResponse> {
    return this.requestJSON<ChatQueryResponse>("/v1/chat/query", {
      method: "POST",
      body: normalizeChatInput(input),
      tenant: tenantOptions
    });
  }

  async streamChatQuery(input: ChatQueryInput, handlers: StreamHandlers = {}, tenantOptions?: TenantRequestOptions): Promise<ChatQueryResponse> {
    const response = await this.fetchImpl(this.buildURL("/v1/chat/query/stream"), {
      method: "POST",
      headers: this.buildHeaders(tenantOptions),
      body: JSON.stringify(normalizeChatInput(input))
    });

    if (!response.ok) {
      throw await toClientError(response);
    }
    if (!response.body) {
      throw new DopeClientError("chat stream response body is missing", { status: 500, code: "stream_body_missing" });
    }

    let terminal: ChatQueryResponse | null = null;
    await readSSE(response.body, (event) => {
      switch (event.event) {
        case "chat.query.started":
          handlers.onStarted?.(JSON.parse(event.data) as ChatQueryStreamStarted);
          break;
        case "chat.query.delta":
          handlers.onDelta?.(JSON.parse(event.data) as ChatQueryStreamDelta);
          break;
        case "chat.query.completed":
          terminal = JSON.parse(event.data) as ChatQueryResponse;
          handlers.onCompleted?.(terminal);
          break;
        case "chat.query.failed":
          terminal = JSON.parse(event.data) as ChatQueryResponse;
          handlers.onFailed?.(terminal);
          break;
        case "chat.query.cancelled":
          terminal = JSON.parse(event.data) as ChatQueryResponse;
          handlers.onCancelled?.(terminal);
          break;
        default:
          break;
      }
    });

    if (!terminal) {
      throw new DopeClientError("chat stream ended without a terminal event", {
        status: 502,
        code: "stream_terminal_event_missing"
      });
    }

    return terminal;
  }

  async getMe(tenantOptions?: TenantRequestOptions): Promise<AuthMeResponse> {
    return this.requestJSON<AuthMeResponse>("/v1/auth/me", { tenant: tenantOptions });
  }

  async listTenants(query: Record<string, QueryValue> = {}, tenantOptions?: TenantRequestOptions): Promise<TenantListResponse> {
    return this.requestJSON<TenantListResponse>("/v1/tenants", { query, tenant: tenantOptions });
  }

  async getTenant(tenantId: string, tenantOptions?: TenantRequestOptions): Promise<TenantDetailResponse> {
    return this.requestJSON<TenantDetailResponse>(`/v1/tenants/${tenantId.trim()}`, { tenant: tenantOptions });
  }

  async listMemberships(tenantId: string, query: Record<string, QueryValue> = {}, tenantOptions?: TenantRequestOptions): Promise<MembershipListResponse> {
    return this.requestJSON<MembershipListResponse>(`/v1/tenants/${tenantId.trim()}/memberships`, {
      query,
      tenant: tenantOptions
    });
  }

  async updateMembershipRole(
    tenantId: string,
    membershipId: string,
    input: UpdateMembershipRoleInput,
    tenantOptions?: TenantRequestOptions
  ): Promise<{ membership: MembershipResource }> {
    return this.requestJSON<{ membership: MembershipResource }>(`/v1/tenants/${tenantId.trim()}/memberships/${membershipId.trim()}`, {
      method: "PATCH",
      body: { role: input.role },
      tenant: tenantOptions
    });
  }

  async removeMembership(tenantId: string, membershipId: string, tenantOptions?: TenantRequestOptions): Promise<{ membership: MembershipResource }> {
    return this.requestJSON<{ membership: MembershipResource }>(`/v1/tenants/${tenantId.trim()}/memberships/${membershipId.trim()}`, {
      method: "DELETE",
      tenant: tenantOptions
    });
  }

  async listTenantSecrets(tenantOptions?: TenantRequestOptions): Promise<TenantSecretListResponse> {
    return this.requestJSON<TenantSecretListResponse>("/v1/tenant-secrets", { tenant: tenantOptions });
  }

  async getTenantSecret(secretRef: string, tenantOptions?: TenantRequestOptions): Promise<TenantSecretResponse> {
    return this.requestJSON<TenantSecretResponse>(`/v1/tenant-secrets/${encodePathComponent(secretRef)}`, { tenant: tenantOptions });
  }

  async getBillingPlan(tenantOptions?: TenantRequestOptions): Promise<BillingPlanResource> {
    return this.requestJSON<BillingPlanResource>("/v1/billing/plan", { tenant: tenantOptions });
  }

  async getBillingUsage(tenantOptions?: TenantRequestOptions): Promise<BillingUsageResponse> {
    return this.requestJSON<BillingUsageResponse>("/v1/billing/usage", { tenant: tenantOptions });
  }

  async listBillingQuotas(tenantOptions?: TenantRequestOptions): Promise<{ items: BillingQuotaResource[] }> {
    return this.requestJSON<{ items: BillingQuotaResource[] }>("/v1/billing/quotas", { tenant: tenantOptions });
  }

  async listBillingDenials(tenantOptions?: TenantRequestOptions): Promise<{ items: BillingDenialResource[] }> {
    return this.requestJSON<{ items: BillingDenialResource[] }>("/v1/billing/denials", { tenant: tenantOptions });
  }

  async assignBillingPlan(tenantId: string, input: BillingPlanAssignmentInput, tenantOptions?: TenantRequestOptions): Promise<BillingPlanResource> {
    return this.requestJSON<BillingPlanResource>(`/v1/admin/billing/tenants/${tenantId.trim()}/plan`, {
      method: "POST",
      body: {
        planKey: input.planKey.trim(),
        enforcementMode: input.enforcementMode,
        reason: input.reason.trim()
      },
      tenant: tenantOptions
    });
  }

  async createBillingQuotaOverride(tenantId: string, input: BillingQuotaOverrideInput, tenantOptions?: TenantRequestOptions): Promise<Record<string, unknown>> {
    return this.requestJSON<Record<string, unknown>>(`/v1/admin/billing/tenants/${tenantId.trim()}/quota-overrides`, {
      method: "POST",
      body: {
        category: input.category,
        limit: input.limit,
        reason: input.reason.trim()
      },
      tenant: tenantOptions
    });
  }

  async createBillingManualAdjustment(tenantId: string, input: BillingManualAdjustmentInput, tenantOptions?: TenantRequestOptions): Promise<BillingManualAdjustmentResource> {
    return this.requestJSON<BillingManualAdjustmentResource>(`/v1/admin/billing/tenants/${tenantId.trim()}/manual-adjustments`, {
      method: "POST",
      body: {
        category: input.category,
        quotaPeriodId: input.quotaPeriodId.trim(),
        amountDelta: input.amountDelta,
        reason: input.reason.trim()
      },
      tenant: tenantOptions
    });
  }

  async resolveBillingReservation(tenantId: string, reservationId: string, input: BillingReservationResolutionInput, tenantOptions?: TenantRequestOptions): Promise<BillingReservationResource> {
    return this.requestJSON<BillingReservationResource>(`/v1/admin/billing/tenants/${tenantId.trim()}/reservations/${reservationId.trim()}/resolve`, {
      method: "POST",
      body: {
        outcome: input.outcome,
        reason: input.reason.trim(),
        amount: input.amount
      },
      tenant: tenantOptions
    });
  }

  async createTenantSecret(input: CreateTenantSecretInput, tenantOptions?: TenantRequestOptions): Promise<TenantSecretResponse> {
    return this.requestJSON<TenantSecretResponse>("/v1/tenant-secrets", {
      method: "POST",
      body: {
        secretRef: input.secretRef.trim(),
        displayName: input.displayName?.trim() || undefined,
        value: input.value,
        document: input.document
      },
      tenant: tenantOptions
    });
  }

  async updateTenantSecret(secretRef: string, input: UpdateTenantSecretInput, tenantOptions?: TenantRequestOptions): Promise<TenantSecretResponse> {
    return this.requestJSON<TenantSecretResponse>(`/v1/tenant-secrets/${encodePathComponent(secretRef)}`, {
      method: "PATCH",
      body: {
        displayName: input.displayName?.trim() || undefined,
        document: input.document
      },
      tenant: tenantOptions
    });
  }

  async rotateTenantSecret(secretRef: string, input: RotateTenantSecretInput, tenantOptions?: TenantRequestOptions): Promise<TenantSecretResponse> {
    return this.requestJSON<TenantSecretResponse>(`/v1/tenant-secrets/${encodePathComponent(secretRef)}/rotate`, {
      method: "POST",
      body: { value: input.value },
      tenant: tenantOptions
    });
  }

  async disableTenantSecret(secretRef: string, input: DisableTenantSecretInput, tenantOptions?: TenantRequestOptions): Promise<TenantSecretResponse> {
    return this.requestJSON<TenantSecretResponse>(`/v1/tenant-secrets/${encodePathComponent(secretRef)}/disable`, {
      method: "POST",
      body: { disabledReason: input.disabledReason.trim() },
      tenant: tenantOptions
    });
  }

  async getOnboarding(tenantOptions?: TenantRequestOptions): Promise<OperatorOnboardingResponse> {
    return this.requestJSON<OperatorOnboardingResponse>("/v1/operator/onboarding", { tenant: tenantOptions });
  }

  async getActivity(query: OperatorActivityQuery = {}, tenantOptions?: TenantRequestOptions): Promise<OperatorActivityListResponse> {
    return this.requestJSON<OperatorActivityListResponse>("/v1/operator/activity", {
      query: {
        sourceKind: query.sourceKind,
        attentionOnly: query.attentionOnly,
        limit: query.limit
      },
      tenant: tenantOptions
    });
  }

  async getDiagnostics(query: OperatorDiagnosticsQuery = {}, tenantOptions?: TenantRequestOptions): Promise<OperatorDiagnosticListResponse> {
    return this.requestJSON<OperatorDiagnosticListResponse>("/v1/operator/diagnostics", {
      query: {
        sourceKind: query.sourceKind,
        plane: query.plane,
        severity: query.severity
      },
      tenant: tenantOptions
    });
  }

  async listReplayCandidates(query: ReplayCandidateQuery = {}, tenantOptions?: TenantRequestOptions): Promise<ReplayCandidateListResponse> {
    return this.requestJSON<ReplayCandidateListResponse>("/v1/evaluation/replay-candidates", { query, tenant: tenantOptions });
  }

  async getReplayCandidate(candidateId: string, tenantOptions?: TenantRequestOptions): Promise<ReplayCandidateResource> {
    return this.requestJSON<ReplayCandidateResource>(`/v1/evaluation/replay-candidates/${candidateId.trim()}`, { tenant: tenantOptions });
  }

  async createReplayCandidate(input: CreateReplayCandidateInput, tenantOptions?: TenantRequestOptions): Promise<ReplayCandidateResource> {
    return this.requestJSON<ReplayCandidateResource>("/v1/evaluation/replay-candidates", {
      method: "POST",
      body: input,
      tenant: tenantOptions
    });
  }

  async createReplayAttempt(candidateId: string, input: CreateReplayAttemptInput = {}, tenantOptions?: TenantRequestOptions): Promise<ReplayAttemptResource> {
    return this.requestJSON<ReplayAttemptResource>(`/v1/evaluation/replay-candidates/${candidateId.trim()}/attempts`, {
      method: "POST",
      body: input,
      tenant: tenantOptions
    });
  }

  async listReplayAttempts(query: ReplayAttemptQuery = {}, tenantOptions?: TenantRequestOptions): Promise<ReplayAttemptListResponse> {
    return this.requestJSON<ReplayAttemptListResponse>("/v1/evaluation/replay-attempts", { query, tenant: tenantOptions });
  }

  async getReplayAttempt(attemptId: string, tenantOptions?: TenantRequestOptions): Promise<ReplayAttemptResource> {
    return this.requestJSON<ReplayAttemptResource>(`/v1/evaluation/replay-attempts/${attemptId.trim()}`, { tenant: tenantOptions });
  }

  async createReplayComparison(attemptId: string, input: CreateReplayComparisonInput = {}, tenantOptions?: TenantRequestOptions): Promise<ReplayComparisonResource> {
    return this.requestJSON<ReplayComparisonResource>(`/v1/evaluation/replay-attempts/${attemptId.trim()}/compare`, {
      method: "POST",
      body: input,
      tenant: tenantOptions
    });
  }

  async listReplayComparisons(query: ReplayComparisonQuery = {}, tenantOptions?: TenantRequestOptions): Promise<ReplayComparisonListResponse> {
    return this.requestJSON<ReplayComparisonListResponse>("/v1/evaluation/comparisons", { query, tenant: tenantOptions });
  }

  async getReplayComparison(comparisonId: string, tenantOptions?: TenantRequestOptions): Promise<ReplayComparisonResource> {
    return this.requestJSON<ReplayComparisonResource>(`/v1/evaluation/comparisons/${comparisonId.trim()}`, { tenant: tenantOptions });
  }

  async listReplayFixtures(query: ReplayFixtureQuery = {}, tenantOptions?: TenantRequestOptions): Promise<ReplayFixtureListResponse> {
    return this.requestJSON<ReplayFixtureListResponse>("/v1/evaluation/fixtures", { query, tenant: tenantOptions });
  }

  async startLiveValidation(input: CreateLiveValidationInput, tenantOptions?: TenantRequestOptions): Promise<CreateLiveValidationResponse> {
    return this.requestJSON<CreateLiveValidationResponse>("/v1/live-validations", {
      method: "POST",
      body: input,
      tenant: tenantOptions
    });
  }

  async listLiveValidations(query: LiveValidationAttemptQuery = {}, tenantOptions?: TenantRequestOptions): Promise<LiveValidationAttemptListResponse> {
    return this.requestJSON<LiveValidationAttemptListResponse>("/v1/live-validations", { query, tenant: tenantOptions });
  }

  async getLiveValidation(validationId: string, tenantOptions?: TenantRequestOptions): Promise<LiveValidationAttemptResource> {
    return this.requestJSON<LiveValidationAttemptResource>(`/v1/live-validations/${validationId.trim()}`, { tenant: tenantOptions });
  }

  async abortLiveValidation(validationId: string, tenantOptions?: TenantRequestOptions): Promise<LiveValidationAttemptResource> {
    return this.requestJSON<LiveValidationAttemptResource>(`/v1/live-validations/${validationId.trim()}/abort`, {
      method: "POST",
      tenant: tenantOptions
    });
  }

  async listLiveValidationSupportMatrix(tenantOptions?: TenantRequestOptions): Promise<LiveValidationSupportMatrixResponse> {
    return this.requestJSON<LiveValidationSupportMatrixResponse>("/v1/live-validations/support-matrix", { tenant: tenantOptions });
  }

  async listLiveValidationLedger(validationId: string, query: { outcome?: LiveValidationLedgerOutcome; toolClass?: string; limit?: number } = {}, tenantOptions?: TenantRequestOptions): Promise<LiveValidationLedgerListResponse> {
    return this.requestJSON<LiveValidationLedgerListResponse>(`/v1/live-validations/${validationId.trim()}/ledger`, { query, tenant: tenantOptions });
  }

  async resolveLiveValidationReconciliation(validationId: string, ambiguousCommitId: string, input: ResolveLiveValidationReconciliationInput, tenantOptions?: TenantRequestOptions): Promise<LiveValidationReconciliationResource> {
    return this.requestJSON<LiveValidationReconciliationResource>(`/v1/live-validations/${validationId.trim()}/reconciliations/${ambiguousCommitId.trim()}/resolve`, {
      method: "POST",
      body: input,
      tenant: tenantOptions
    });
  }

  async getLiveValidationRetention(validationId: string, tenantOptions?: TenantRequestOptions): Promise<LiveValidationRetentionResource> {
    return this.requestJSON<LiveValidationRetentionResource>(`/v1/live-validations/${validationId.trim()}/retention`, { tenant: tenantOptions });
  }

  async createLiveValidationComparison(validationId: string, tenantOptions?: TenantRequestOptions): Promise<LiveValidationComparisonResource> {
    return this.requestJSON<LiveValidationComparisonResource>(`/v1/live-validations/${validationId.trim()}/compare`, {
      method: "POST",
      tenant: tenantOptions
    });
  }

  async listLiveValidationKillSwitches(query: { tenantId?: string; scope?: "tenant" | "global"; limit?: number } = {}, tenantOptions?: TenantRequestOptions): Promise<LiveValidationKillSwitchListResponse> {
    return this.requestJSON<LiveValidationKillSwitchListResponse>("/v1/live-validations/kill-switches", { query, tenant: tenantOptions });
  }

  async updateLiveValidationKillSwitch(input: UpdateLiveValidationKillSwitchInput, tenantOptions?: TenantRequestOptions): Promise<LiveValidationKillSwitchResource> {
    return this.requestJSON<LiveValidationKillSwitchResource>("/v1/live-validations/kill-switches", {
      method: "POST",
      body: input,
      tenant: tenantOptions
    });
  }

  async listApprovals(status?: ApprovalStatus, tenantOptions?: TenantRequestOptions): Promise<ApprovalListResponse> {
    return this.requestJSON<ApprovalListResponse>("/v1/policy/approvals", {
      query: { status },
      tenant: tenantOptions
    });
  }

  async getApproval(approvalId: string, tenantOptions?: TenantRequestOptions): Promise<ApprovalResource> {
    return this.requestJSON<ApprovalResource>(`/v1/policy/approvals/${approvalId.trim()}`, { tenant: tenantOptions });
  }

  async resolveApproval(approvalId: string, input: ResolveApprovalInput, tenantOptions?: TenantRequestOptions): Promise<ApprovalDecisionResponse> {
    return this.requestJSON<ApprovalDecisionResponse>(`/v1/policy/approvals/${approvalId.trim()}/resolve`, {
      method: "POST",
      body: {
        resolution: input.resolution,
        comment: input.comment?.trim() || ""
      },
      tenant: tenantOptions
    });
  }

  async createRun(input: CreateRunInput, tenantOptions?: TenantRequestOptions): Promise<RunResource> {
    return this.requestJSON<RunResource>("/v1/runs", {
      method: "POST",
      body: {
        sessionId: input.sessionId?.trim() || undefined,
        route: input.route,
        entrypoint: input.entrypoint.trim(),
        goal: input.goal?.trim() || undefined,
        input: input.input
      },
      tenant: tenantOptions
    });
  }

  async fetchRoute<T>(route: string, tenantOptions?: TenantRequestOptions): Promise<T> {
    return this.requestJSON<T>(normalizeRoute(route), { tenant: tenantOptions });
  }

  streamEvents(query: EventStreamQuery = {}, handlers: EventStreamHandlers = {}, tenantOptions?: TenantRequestOptions): EventStreamSubscription {
    const controller = new AbortController();
    const completed = (async () => {
      const response = await this.fetchImpl(this.buildURL("/v1/events/stream", query), {
        method: "GET",
        headers: this.buildHeaders(tenantOptions),
        signal: controller.signal
      });

      if (!response.ok) {
        throw await toClientError(response);
      }
      if (!response.body) {
        throw new DopeClientError("event stream response body is missing", { status: 500, code: "stream_body_missing" });
      }

      await readSSE(response.body, (event) => {
        handlers.onEvent?.(JSON.parse(event.data) as DaemonEvent);
      });
    })().catch((error: unknown) => {
      if (controller.signal.aborted) {
        return;
      }
      handlers.onError?.(error instanceof Error ? error : new Error(String(error)));
    });

    return {
      close() {
        controller.abort();
      },
      completed
    };
  }

  private async requestJSON<T>(
    route: string,
    options: {
      method?: string;
      query?: Record<string, QueryValue>;
      body?: unknown;
      tenant?: TenantRequestOptions;
    } = {}
  ): Promise<T> {
    const response = await this.fetchImpl(this.buildURL(route, options.query), {
      method: options.method ?? "GET",
      headers: this.buildHeaders(options.tenant),
      body: options.body === undefined ? undefined : JSON.stringify(options.body)
    });

    if (!response.ok) {
      throw await toClientError(response);
    }
    return (await response.json()) as T;
  }

  private buildURL(route: string, query?: Record<string, QueryValue>): string {
    const url = new URL(`${this.baseURL}${normalizeRoute(route)}`);
    if (!query) {
      return url.toString();
    }

    for (const [key, value] of Object.entries(query)) {
      if (value === undefined || value === null || value === "") {
        continue;
      }
      url.searchParams.set(key, String(value));
    }
    return url.toString();
  }

  private buildHeaders(tenantOptions?: TenantRequestOptions): HeadersInit {
    const headers: Record<string, string> = {
      "Content-Type": "application/json"
    };
    if (this.accessToken) {
      headers.Authorization = `Bearer ${this.accessToken}`;
    }
    const tenantId = this.resolveTenantId(tenantOptions);
    if (tenantId) {
      headers["X-Dope-Tenant-ID"] = tenantId;
    }
    return headers;
  }

  private resolveTenantId(tenantOptions?: TenantRequestOptions): string | undefined {
    return tenantOptions?.tenantId?.trim() || this.defaultTenantId;
  }
}

export function createDopeClient(options: DopeClientOptions): DopeClient {
  return new DopeClient(options);
}

type QueryValue = string | number | boolean | undefined | null;

function trimBaseURL(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    throw new Error("baseURL is required");
  }
  return trimmed.replace(/\/+$/, "");
}

function normalizeRoute(route: string): string {
  const trimmed = route.trim();
  if (!trimmed) {
    throw new Error("route is required");
  }
  return trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
}

function encodePathComponent(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    throw new Error("path value is required");
  }
  return encodeURIComponent(trimmed);
}

function normalizeChatInput(input: ChatQueryInput): ChatQueryInput {
  return {
    provider: input.provider?.trim() || undefined,
    model: input.model?.trim() || undefined,
    skills: input.skills?.map((item) => item.trim()).filter(Boolean),
    query: input.query.trim(),
    timeoutMs: input.timeoutMs,
    maxRetries: input.maxRetries
  };
}

async function toClientError(response: Response): Promise<DopeClientError> {
  let message = `request failed with status ${response.status}`;
  let code: string | undefined;
  let denial: TenantDenialResource | undefined;
  let quotaDenial: BillingQuotaDenialPayload | undefined;

  try {
    const payload = (await response.json()) as {
      error?: string;
      errorCode?: string;
      requestId?: string;
    } & Partial<BillingQuotaDenialPayload>;
    if (payload.error) {
      message = payload.error;
    }
    if (payload.errorCode || payload.reasonCode) {
      code = payload.errorCode ?? payload.reasonCode;
    }
    if (isTenantDenialCode(payload.errorCode) && payload.errorCode) {
      denial = {
        error: payload.error ?? "tenant access denied",
        errorCode: payload.errorCode,
        requestId: payload.requestId
      };
    }
    if (payload.code === "quota_denied" && payload.reasonCode) {
      quotaDenial = {
        code: "quota_denied",
        reasonCode: payload.reasonCode,
        tenantId: payload.tenantId,
        category: payload.category,
        operationKey: payload.operationKey,
        requestedAmount: payload.requestedAmount,
        remainingAmount: payload.remainingAmount,
        periodStart: payload.periodStart,
        periodEnd: payload.periodEnd
      };
      code = payload.reasonCode;
    }
  } catch {
    // Ignore non-json failure bodies.
  }

  const tenantDenied = Boolean(denial) || isTenantDenialCode(code);
  return new DopeClientError(message, { status: response.status, code, tenantDenied: tenantDenied || undefined, denial, quotaDenial });
}

function isTenantDenialCode(code: string | undefined): boolean {
  return code === "tenant_access_denied" || code === "tenant_permission_denied" || code === "tenant_ownership_denied" || code?.startsWith("credential_denied:") === true;
}

type SSEEvent = {
  event: string;
  data: string;
};

async function readSSE(stream: ReadableStream<Uint8Array>, onEvent: (event: SSEEvent) => void | Promise<void>): Promise<void> {
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }
    buffer += decoder.decode(value, { stream: true });
    const parts = buffer.split("\n\n");
    buffer = parts.pop() ?? "";
    for (const part of parts) {
      const parsed = parseSSEEvent(part);
      if (parsed) {
        await onEvent(parsed);
      }
    }
  }

  buffer += decoder.decode();
  if (buffer.trim()) {
    const parsed = parseSSEEvent(buffer);
    if (parsed) {
      await onEvent(parsed);
    }
  }
}

function parseSSEEvent(chunk: string): SSEEvent | null {
  const lines = chunk
    .split("\n")
    .map((line) => line.trimEnd())
    .filter((line) => line !== "");

  let event = "";
  const data: string[] = [];

  for (const line of lines) {
    if (line.startsWith(":")) {
      continue;
    }
    if (line.startsWith("event:")) {
      event = line.slice("event:".length).trim();
      continue;
    }
    if (line.startsWith("data:")) {
      data.push(line.slice("data:".length).trim());
    }
  }

  if (!event || data.length === 0) {
    return null;
  }

  return {
    event,
    data: data.join("\n")
  };
}
