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

export type EvaluationProductResourceKind =
  | "discovery_policy"
  | "discovery_run"
  | "discovered_candidate"
  | "candidate_evidence"
  | "suppression"
  | "product_fixture"
  | "fixture_revision"
  | "campaign"
  | "campaign_item"
  | "campaign_attempt_group"
  | "dashboard_projection"
  | "tool_call_inspection"
  | "retention_application";

export type EvaluationProductLifecycleStatus =
  | "queued"
  | "running"
  | "completed"
  | "partial"
  | "failed"
  | "cancelled"
  | "draft"
  | "in_review"
  | "approved"
  | "rejected"
  | "published"
  | "archived"
  | "deleted"
  | "expired"
  | "suppressed";

export type EvaluationProductRetentionState = "active" | "expired" | "deleted" | "tombstone";
export type EvaluationProductSuppressionState = "none" | "suppressed" | "expired" | "revoked";
export type EvaluationProductRedactionStatus = "clean" | "redacted" | "failed";
export type EvaluationProductScoreBand = "high" | "medium" | "low";

export type EvaluationProductTenantOptions = {
  tenantId?: string;
  cursor?: string;
  limit?: number;
};

export type EvaluationProductCursorPage = {
  cursor?: string;
  nextCursor?: string;
  limit: number;
};

export type EvaluationProductDenial = {
  code: string;
  message: string;
  reasonCode: string;
  permission?: string;
  tenantId?: string;
  resourceKind?: EvaluationProductResourceKind;
  resourceId?: string;
  checkedAt?: string;
};

export type EvaluationProductRedactionMetadata = {
  status: EvaluationProductRedactionStatus;
  rulesApplied?: string[];
  sensitiveFieldsExcluded?: string[];
  failureReasonCode?: string;
};

export type EvaluationProductRetentionOutcome = {
  applicationId: string;
  tenantId: string;
  resourceKind: EvaluationProductResourceKind;
  resourceId?: string;
  dryRun: boolean;
  outcome: "retained" | "expired" | "deleted" | "tombstoned" | "dry_run" | "failed";
  reasonCode?: string;
  affectedCount?: number;
  appliedAt: string;
};

export type EvaluationDiscoveryPolicyResource = {
  policyId: string;
  tenantId: string;
  enabled: boolean;
  sourceKinds: ReplayCandidateResource["sourceKind"][];
  windowStart: string;
  windowEnd: string;
  maxInspectedRecords: number;
  maxEmittedCandidates: number;
  costBudget: number;
  sensitiveFieldRules?: string[];
  retentionPolicyRef?: string;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export type EvaluationDiscoveryRunResource = {
  discoveryRunId: string;
  tenantId: string;
  policyId?: string;
  status: EvaluationProductLifecycleStatus;
  cursor?: string;
  sourceKinds: ReplayCandidateResource["sourceKind"][];
  windowStart: string;
  windowEnd: string;
  maxInspectedRecords: number;
  maxEmittedCandidates: number;
  costBudget: number;
  inspectedRecords: number;
  emittedCandidates: number;
  partialReason?: string;
  startedBy?: string;
  startedAt: string;
  completedAt?: string;
  updatedAt: string;
  idempotencyKey?: string;
};

export type EvaluationDiscoveredCandidateResource = {
  discoveredCandidateId: string;
  tenantId: string;
  discoveryRunId: string;
  sourceKind: ReplayCandidateResource["sourceKind"];
  sourceId: string;
  sourceRefs?: ReplaySourceRef[];
  score: number;
  scoreBand: EvaluationProductScoreBand;
  explanationFields?: Record<string, unknown>;
  redactionStatus: EvaluationProductRedactionStatus;
  evidenceRef?: string;
  readinessStatus: ReplayCandidateResource["readinessStatus"];
  suppressionState: EvaluationProductSuppressionState;
  retentionState: EvaluationProductRetentionState;
  createdAt: string;
  updatedAt: string;
  expiresAt?: string;
};

export type EvaluationSuppressionRecord = {
  suppressionId: string;
  tenantId: string;
  targetKind: EvaluationProductResourceKind;
  targetId?: string;
  targetSourceRef?: string;
  reasonCode: string;
  reason?: string;
  createdBy?: string;
  createdAt: string;
  expiresAt?: string;
  active: boolean;
};

export type EvaluationProductListResponse<T> = {
  tenantId: string;
  page: EvaluationProductCursorPage;
  items: T[];
};

export type EvaluationDiscoveryPolicyQuery = {
  enabled?: boolean;
  cursor?: string;
  limit?: number;
};

export type UpsertEvaluationDiscoveryPolicyInput = {
  enabled: boolean;
  sourceKinds: ReplayCandidateResource["sourceKind"][];
  windowStart: string;
  windowEnd: string;
  maxInspectedRecords: number;
  maxEmittedCandidates: number;
  costBudget: number;
  sensitiveFieldRules?: string[];
  retentionPolicyRef?: string;
  idempotencyKey?: string;
};

export type StartEvaluationDiscoveryRunInput = {
  policyId?: string;
  windowStart?: string;
  windowEnd?: string;
  sourceKinds?: ReplayCandidateResource["sourceKind"][];
  maxInspectedRecords?: number;
  maxEmittedCandidates?: number;
  costBudget?: number;
  cursor?: string;
  idempotencyKey?: string;
};

export type EvaluationDiscoveryRunQuery = {
  status?: EvaluationDiscoveryRunResource["status"];
  sourceKind?: ReplayCandidateResource["sourceKind"];
  cursor?: string;
  limit?: number;
};

export type EvaluationDiscoveredCandidateQuery = {
  discoveryRunId?: string;
  sourceKind?: ReplayCandidateResource["sourceKind"];
  readinessStatus?: ReplayCandidateResource["readinessStatus"];
  suppressionState?: EvaluationProductSuppressionState;
  scoreBand?: EvaluationProductScoreBand;
  cursor?: string;
  limit?: number;
};

export type CreateEvaluationSuppressionInput = {
  suppressionId?: string;
  targetKind: EvaluationProductResourceKind;
  targetId?: string;
  targetSourceRef?: string;
  reasonCode: string;
  reason?: string;
  expiresAt?: string;
  idempotencyKey?: string;
};

export type ProductFixtureDomainClass = "schedule" | "integration" | "computer_use";

export type ProductFixtureResource = {
  fixtureId: string;
  tenantId: string;
  displayName: string;
  domainClass: ProductFixtureDomainClass;
  sourceKind: string;
  sourceRefs?: ReplaySourceRef[];
  sourceCandidateId?: string;
  currentRevisionId: string;
  reviewState: EvaluationProductLifecycleStatus;
  suppressionState: EvaluationProductSuppressionState;
  retentionState: EvaluationProductRetentionState;
  createdBy?: string;
  createdAt: string;
  updatedAt: string;
};

export type FixtureRevisionResource = {
  revisionId: string;
  fixtureId: string;
  tenantId: string;
  revisionNumber: number;
  contentSummary?: string;
  fixturePayload?: Record<string, unknown>;
  changeSummary?: string;
  sourceEvidenceRefs?: string[];
  redactionStatus: EvaluationProductRedactionStatus;
  createdBy?: string;
  createdAt: string;
};

export type ProductFixtureMutationResponse = {
  fixture: ProductFixtureResource;
  revision?: FixtureRevisionResource;
};

export type ProductFixtureQuery = {
  domainClass?: ProductFixtureDomainClass;
  reviewState?: EvaluationProductLifecycleStatus;
  suppressionState?: EvaluationProductSuppressionState;
  cursor?: string;
  limit?: number;
};

export type CreateProductFixtureInput = {
  fixtureId?: string;
  displayName: string;
  domainClass: ProductFixtureDomainClass;
  fixturePayload?: Record<string, unknown>;
  changeSummary?: string;
  idempotencyKey?: string;
};

export type CreateFixtureRevisionInput = {
  revisionId?: string;
  fixturePayload?: Record<string, unknown>;
  contentSummary?: string;
  changeSummary?: string;
  sourceEvidenceRefs?: string[];
  idempotencyKey?: string;
};

export type ReviewProductFixtureInput = {
  revisionId: string;
  decision: "approved" | "rejected" | "needs_changes";
  reason?: string;
};

export type EvaluationCampaignResource = {
  campaignId: string;
  tenantId: string;
  displayName: string;
  status: EvaluationProductLifecycleStatus;
  scopeSummary?: string;
  startedBy?: string;
  createdAt: string;
  startedAt?: string;
  completedAt?: string;
  publishedAt?: string;
  retentionState: EvaluationProductRetentionState;
  idempotencyKey?: string;
};

export type EvaluationCampaignItemResource = {
  campaignItemId: string;
  campaignId: string;
  tenantId: string;
  sourceType: EvaluationProductResourceKind;
  sourceId: string;
  sourceSnapshot?: Record<string, unknown>;
  selectionReason?: string;
  suppressionCheckedAt: string;
  createdAt: string;
};

export type EvaluationCampaignAttemptGroupResource = {
  attemptGroupId: string;
  campaignId: string;
  campaignItemId: string;
  tenantId: string;
  replayAttemptIds?: string[];
  comparisonIds?: string[];
  liveValidationIds?: string[];
  status: EvaluationProductLifecycleStatus;
  driftCount: number;
  failureCount: number;
  unsupportedCount: number;
  operatorActionNeededCount: number;
  summary?: string;
  createdAt: string;
  updatedAt: string;
};

export type CreateEvaluationCampaignInput = {
  campaignId?: string;
  displayName: string;
  scopeSummary?: string;
  sourceSelections?: Array<{
    sourceType: EvaluationProductResourceKind;
    sourceId: string;
    sourceSnapshot?: Record<string, unknown>;
    selectionReason?: string;
  }>;
  startImmediately?: boolean;
  idempotencyKey?: string;
};

export type EvaluationDashboardProjectionResource = {
  projectionId: string;
  tenantId: string;
  windowStart: string;
  windowEnd: string;
  campaignStatusCounts?: Record<string, number>;
  driftSummary?: Record<string, number>;
  failureSummary?: Record<string, number>;
  unsupportedSummary?: Record<string, number>;
  operatorActionNeededSummary?: Record<string, number>;
  liveValidationSummary?: Record<string, number>;
  candidateSummary?: Record<string, number>;
  fixtureSummary?: Record<string, number>;
  generatedAt: string;
  cursor?: string;
  retentionState?: EvaluationProductRetentionState;
};

export type EvaluationToolCallInspectionClassification =
  | "matched"
  | "drifted"
  | "failed"
  | "unsupported"
  | "missing_original_evidence"
  | "missing_replay_evidence"
  | "live_validation_denied"
  | "live_validation_aborted"
  | "live_validation_failed"
  | "live_validation_operator_action_needed"
  | "live_validation_completed";

export type EvaluationToolCallInspectionResource = {
  inspectionId: string;
  tenantId: string;
  campaignId: string;
  campaignItemId: string;
  toolCallRef: string;
  originalEvidenceRef?: string;
  nonLiveReplayEvidenceRef?: string;
  liveValidationLedgerRefs?: string[];
  classification: EvaluationToolCallInspectionClassification;
  diffSummary?: string;
  redactionStatus: EvaluationProductRedactionStatus;
  retentionState?: EvaluationProductRetentionState;
  createdAt: string;
  updatedAt: string;
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
  | "integrations.diagnostics.read"
  | "integrations.diagnostics.run"
  | "integrations.diagnostics.smoke"
  | "integrations.diagnostics.smoke_risky"
  | "connectors.manage"
  | "mcp.manage"
  | "runs.execute"
  | "approvals.resolve"
  | "live_validation.execute"
  | "live_validation.reconcile"
  | "evaluation.manage"
  | "billing.view"
  | "billing.manage"
  | "billing.evidence_export"
  | "read_only.inspect";

export type IntegrationDiagnosticStatus = "unknown" | "healthy" | "degraded" | "blocked" | "unsupported";
export type IntegrationDiagnosticFreshnessState = "fresh" | "stale";
export type IntegrationDiagnosticRetrySafety = "no_action_needed" | "retryable" | "blocked" | "unsafe_to_retry" | "operator_action_needed";
export type IntegrationDiagnosticRemediationOwner = "product_user" | "tenant_admin" | "operator" | "provider" | "none_required";
export type IntegrationDiagnosticRedactionStatus = "redacted" | "suppressed" | "failed_closed";

export type IntegrationDiagnosticReasonCode =
  | "healthy"
  | "auth_missing"
  | "permission_missing"
  | "blocked_route"
  | "duplicate_inbound"
  | "reply_failed"
  | "unsupported_capability"
  | "unknown_connector_failure"
  | "app_authorization_missing"
  | "bot_authorization_missing"
  | "user_authorization_missing"
  | "tenant_approval_pending"
  | "scope_missing"
  | "token_missing"
  | "token_expired"
  | "token_revoked"
  | "refresh_credentials_missing"
  | "token_refresh_failed"
  | "tenant_mismatch"
  | "rate_limited"
  | "provider_unavailable"
  | "transient_provider_failure"
  | "network_failed"
  | "ambiguous_downstream_commit"
  | "unsafe_to_retry"
  | "operator_action_needed"
  | "limited_diagnostic"
  | "unsupported_diagnostic"
  | "redaction_failed_closed"
  | "unknown_provider_error";

export type DiscordReadinessState = "hosted_ready" | "degraded_needs_repair" | "failed" | "disabled";
export type DiscordCredentialState = "missing" | "submitted" | "valid" | "invalid" | "revoked" | "redaction_suppressed";
export type DiscordRedactionStatus = "redacted" | "suppressed" | "redaction_failed";

export type DiscordDestinationValidationResource = {
  tenantId?: string;
  connectorId: string;
  destinationId: string;
  destinationType: "guild" | "channel" | "direct_message";
  providerLabel?: string;
  selected: boolean;
  validationState: "valid" | "invalid" | "missing_permission" | "message_content_missing" | "bot_not_member" | "not_found" | "dm_restricted" | "stale";
  reasonCode?: string;
  validatedAt: string;
  redactionStatus: DiscordRedactionStatus;
  safeEvidence?: Record<string, string>;
};

export type DiscordHostedSetupResource = {
  tenantId?: string;
  connectorId: string;
  connectorKind: "discord";
  displayName: string;
  status: "configured" | "healthy" | "degraded" | "failed" | "permission_blocked" | "rate_limited" | "unsupported_capability";
  readinessState: DiscordReadinessState;
  hostedReady: boolean;
  credentialState: DiscordCredentialState;
  respondInDM: boolean;
  requireMention: boolean;
  deliveryMode: "gateway";
  reasonCode?: string;
  destinations?: DiscordDestinationValidationResource[];
  redactionStatus: DiscordRedactionStatus;
  createdAt: string;
  updatedAt: string;
  validatedAt?: string;
  retentionExpiresAt: string;
};

export type DiscordSmokeEvidenceResource = {
  smokeEvidenceId: string;
  tenantId?: string;
  connectorId: string;
  status: "passed" | "failed" | "skipped";
  credentialMode: "safe_live" | "fake" | "unavailable";
  owner: string;
  reason: string;
  remainingRisk?: string;
  validatedAt: string;
  retentionExpiresAt: string;
  redactionStatus: DiscordRedactionStatus;
  safeEvidence?: Record<string, string>;
};

export type ConnectorConformanceResultResource = {
  conformanceResultId: string;
  tenantId?: string;
  connectorKind: string;
  connectorId?: string;
  scenarioId: string;
  area: string;
  result: "pass" | "fail" | "supported" | "limited" | "unsupported";
  reasonCode?: string;
  redactionStatus: DiscordRedactionStatus;
  evidenceTimestamp: string;
  retentionExpiresAt: string;
};

export type DiscordConformanceEvidenceResponse = {
  tenantId: string;
  connectorId: string;
  items: ConnectorConformanceResultResource[];
};

export type DiscordHostedReadinessProjection = {
  tenantId?: string;
  connectorId: string;
  displayName: string;
  deliveryMode: "gateway";
  readinessState: DiscordReadinessState;
  hostedReady: boolean;
  localCompatible: boolean;
  reasonCode?: string;
  requireMention: boolean;
  respondInDM: boolean;
  allowedGuildIds?: string[];
  allowedChannelIds?: string[];
  botTokenConfigured: boolean;
  redactionStatus: DiscordRedactionStatus;
};

export type ConfigDiscordConnectorResponse = {
  enabled: boolean;
  configured: boolean;
  connectorId: string;
  displayName: string;
  deliveryMode: "gateway";
  requireMention: boolean;
  respondInDM: boolean;
  allowedGuildIds: string[];
  allowedChannelIds: string[];
  botTokenConfigured: boolean;
  botTokenEnv?: string;
  hostedReadiness: DiscordHostedReadinessProjection;
};

export type ConfigResponse = {
  environment: EnvironmentScope;
  bindAddr: string;
  dataDir: string;
  configFilePath: string;
  logLevel: "debug" | "info" | "warn" | "error";
  version: string;
  llm: Record<string, unknown>;
  connectors: {
    discord: ConfigDiscordConnectorResponse;
  };
  mcp: Record<string, unknown>;
  sandbox: Record<string, unknown>;
  redactedFields: string[];
};

export type IntegrationDiagnosticResultResource = {
  diagnosticResultId: string;
  tenantId: string;
  integrationId: string;
  integrationAccountId?: string;
  domainKind: string;
  providerKind: string;
  capability: string;
  status: IntegrationDiagnosticStatus;
  reasonCode: IntegrationDiagnosticReasonCode;
  remediationOwner: IntegrationDiagnosticRemediationOwner;
  remediationHint?: string;
  retrySafety: IntegrationDiagnosticRetrySafety;
  checkedAt: string;
  staleAfter: string;
  freshnessState: IntegrationDiagnosticFreshnessState;
  runId?: string;
  redactionStatus: IntegrationDiagnosticRedactionStatus;
  evidenceSummary?: string;
  retentionExpiresAt: string;
  smokeReportId?: string;
  artifactRefs?: string[];
};

export type IntegrationDiagnosticRunResource = {
  diagnosticRunId: string;
  tenantId: string;
  integrationId: string;
  integrationAccountId?: string;
  domainKind?: string;
  providerKind?: string;
  requestedBy: string;
  trigger: string;
  status: "queued" | "running" | "completed" | "failed" | "blocked";
  startedAt: string;
  completedAt?: string;
  checkedCapabilities: string[];
  resultIds: string[];
  failureReasonCode?: IntegrationDiagnosticReasonCode;
  redactionStatus: IntegrationDiagnosticRedactionStatus;
  retentionExpiresAt: string;
  idempotencyKey?: string;
};

export type IntegrationDiagnosticReasonCodeResource = {
  reasonCode: IntegrationDiagnosticReasonCode;
  category: string;
  defaultRetrySafety: IntegrationDiagnosticRetrySafety;
  defaultRemediationOwner: IntegrationDiagnosticRemediationOwner;
  userMessageKey: string;
  operatorMessageKey: string;
  supportedDomains?: string[];
};

export type IntegrationDiagnosticListResponse = {
  integrationId?: string;
  tenantId?: string;
  freshnessSummary?: string;
  items: IntegrationDiagnosticResultResource[];
  nextCursor?: string;
};

export type CreateIntegrationDiagnosticRunInput = {
  capabilities?: string[];
  forceRefresh?: boolean;
  clientKey: string;
  reason?: string;
};

export type SmokeProbeResult = "passed" | "failed" | "blocked" | "skipped";
export type SmokeReportStatus = "draft" | "running" | "completed" | "blocked" | "failed" | "published";

export type SmokeProbeOutcomeResource = {
  probeOutcomeId: string;
  tenantId: string;
  smokeReportId: string;
  integrationId: string;
  integrationAccountId?: string;
  domainKind: string;
  providerKind: string;
  probeAction: string;
  result: SmokeProbeResult;
  reasonCode: IntegrationDiagnosticReasonCode;
  remediationHint: string;
  retrySafety: IntegrationDiagnosticRetrySafety;
  blockedOrSkippedReason?: string;
  approvalRefs?: string[];
  artifactRefs?: string[];
  checkedAt: string;
  redactionStatus: IntegrationDiagnosticRedactionStatus;
  retentionExpiresAt: string;
};

export type SmokeMatrixReportResource = {
  smokeReportId: string;
  tenantId: string;
  reportKind: string;
  requestedBy: string;
  status: SmokeReportStatus;
  domainSummary: Record<string, string>;
  startedAt: string;
  completedAt?: string;
  publishedAt?: string;
  artifactRefs: string[];
  retentionExpiresAt: string;
  probeOutcomes?: SmokeProbeOutcomeResource[];
};

export type CreateIntegrationDiagnosticSmokeInput = {
  reportId?: string;
  integrationId: string;
  probes?: Array<{
    integrationId?: string;
    domainKind?: string;
    probeAction: string;
    safeCredentialsAvailable: boolean;
    tenantApprovalAvailable: boolean;
    providerAvailable: boolean;
    supported: boolean;
    readOnlyOrReversible: boolean;
    tenantAdminApproved?: boolean;
    operatorApproved?: boolean;
    operatorDeferred?: boolean;
    reasonCode?: IntegrationDiagnosticReasonCode;
    providerEvidence?: Record<string, unknown>;
    artifactRefs?: string[];
  }>;
};

export type DiagnosticRetentionRecordResource = {
  retentionRecordId: string;
  tenantId: string;
  targetKind: string;
  targetId: string;
  policyRef?: string;
  defaultExpiresAt: string;
  effectiveExpiresAt: string;
  retentionState: "active" | "expired" | "purged";
  appliedAt?: string;
  createdAt: string;
  updatedAt: string;
};

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

function isTenantRequestOptions(value: unknown): value is TenantRequestOptions {
  return typeof value === "object" && value !== null && "tenantId" in value;
}

export type ActivationStatus = "not_started" | "in_progress" | "blocked" | "active" | "first_action_completed";

export type ActivationReasonCode =
  | "activation_denied:principal_disabled"
  | "activation_denied:principal_denied"
  | "activation_denied:tenant_access_revoked"
  | "activation_blocked:quota_baseline_unavailable"
  | "activation_blocked:environment_unavailable"
  | "activation_blocked:test_chat_unavailable"
  | "activation_failed:tenant_resolution"
  | "activation_failed:test_chat"
  | "activation_failed:audit_write"
  | "activation_failed:persistence"
  | "activation_failed:unexpected"
  | string;

export type ActivationRemediationOwner = "product_user" | "operator" | "tenant_admin" | "system" | "none_required";

export type ActivationReadinessItem = {
  itemId: string;
  itemKind: "tenant_access" | "environment" | "quota_baseline" | "test_chat" | string;
  status: "ready" | "blocked" | "degraded" | "missing_configuration" | "optional";
  reasonCode?: ActivationReasonCode;
  displayName?: string;
  requiredForActivation: boolean;
  retryable: boolean;
  remediationOwner: ActivationRemediationOwner;
  updatedAt?: string;
};

export type ActivationQuotaProjection = {
  category?: string;
  unit?: string;
  limit?: number;
  used?: number;
  remaining?: number;
  period?: string;
  metadata?: Record<string, unknown>;
};

export type ActivationQuotaBaseline = {
  tenantId: string;
  planKey: string;
  enforcementMode: BillingEnforcementMode | string;
  status: "available" | "unavailable";
  quotas: ActivationQuotaProjection[];
  projectedAt?: string;
  projectionSource?: string;
  reasonCode?: ActivationReasonCode;
};

export type ActivationFirstAction = {
  actionId: "test_chat" | string;
  actionKind: "test_chat" | string;
  displayName?: string;
  recommended: boolean;
  available: boolean;
  blockingItemIds: string[];
  invokeRoute: string;
  resultRoute: string;
};

export type ActivationTestChatMetadata = {
  activationId?: string;
  tenantId?: string;
  dispatchId?: string;
  status: "completed" | "failed" | "cancelled";
  provider?: string;
  model?: string;
  usage?: Record<string, unknown>;
  finishReason?: string;
  reasonCode?: ActivationReasonCode;
  completedAt?: string;
};

export type ActivationStateResource = {
  activationId: string;
  principalId: string;
  tenantId: string;
  environmentScope: EnvironmentScope;
  status: ActivationStatus;
  currentStepId: string;
  completedStepIds: string[];
  blockingReasonCodes: ActivationReasonCode[];
  readinessItems: ActivationReadinessItem[];
  quotaBaseline?: ActivationQuotaBaseline;
  firstAction: ActivationFirstAction;
  testChat?: ActivationTestChatMetadata;
  failureReason?: {
    reasonCode: ActivationReasonCode;
    stage: ActivationDiagnostic["stage"];
    retryable: boolean;
    remediationOwner: ActivationRemediationOwner;
    message?: string;
  };
  createdAt?: string;
  updatedAt?: string;
  firstActionCompletedAt?: string;
  lastEvaluatedAt: string;
};

export type ActivationResponse = {
  activation: ActivationStateResource;
};

export type RunActivationInput = {
  source?: "signup" | "invite_acceptance" | "returning_user" | "operator_retry" | string;
};

export type RunActivationTestChatInput = {
  message: string;
};

export type ActivationTestChatResponse = ActivationResponse & {
  testChat: ActivationTestChatMetadata;
};

export type ActivationDiagnostic = {
  activationId: string;
  tenantId?: string;
  principalId: string;
  status: ActivationStatus;
  stage: "tenant_resolution" | "eligibility" | "quota_baseline" | "authorization" | "test_chat" | "audit" | "persistence" | "unexpected" | string;
  reasonCode: ActivationReasonCode;
  retryable: boolean;
  remediationOwner: ActivationRemediationOwner;
  lastTransitionAt: string;
  readinessItemIds?: string[];
  quotaBaselineStatus?: "available" | "unavailable";
  testChat?: ActivationTestChatMetadata;
};

export type ActivationDiagnosticListResponse = {
  items: ActivationDiagnostic[];
};

export type ActivationFailurePayload = {
  code?: ActivationReasonCode;
  reasonCode: ActivationReasonCode;
  stage: ActivationDiagnostic["stage"];
  retryable: boolean;
  remediationOwner: ActivationRemediationOwner;
};

export type SetupState = "not_started" | "in_progress" | "ready" | "degraded" | "unavailable" | "cancelled" | "action_required" | "disabled";
export type SetupStyle = "submitted_secret" | "oauth" | "unsupported";
export type SetupTargetKind = "provider" | "integration" | "channel" | "connector";
export type SetupSupportStatus = "supported" | "unsupported" | "action_required";
export type SetupSafeUseMode = "normal" | "limited_safe" | "blocked";
export type SetupRemediationOwner = "product_user" | "tenant_admin" | "operator" | "provider" | "none_required";
export type SetupRedactionStatus = "redacted" | "suppressed" | "failed_closed";
export type SetupRetrySafety = "no_action_needed" | "retryable" | "blocked" | "unsafe_to_retry";
export type SetupOAuthResult = "completed" | "denied" | "abandoned" | "expired" | "replay" | "tenant_mismatch" | "provider_error";

export type SetupResourceRef = {
  kind: string;
  id: string;
  route?: string;
};

export type SetupTargetResource = {
  targetId: string;
  tenantId?: string;
  targetKind: SetupTargetKind;
  setupStyle: SetupStyle;
  displayName: string;
  proofTarget: boolean;
  supportStatus: SetupSupportStatus;
  requiredPermissions?: TenantPermission[];
  limitedSafeCapabilities?: string[];
  currentSessionId?: string;
  currentState?: SetupState;
  diagnosticResultId?: string;
};

export type SetupSessionResource = {
  setupSessionId: string;
  tenantId?: string;
  actorPrincipalId?: string;
  targetId: string;
  targetKind: SetupTargetKind;
  setupStyle: SetupStyle;
  state: SetupState;
  reasonCode?: string;
  retryable: boolean;
  remediationOwner: SetupRemediationOwner;
  safeUseMode: SetupSafeUseMode;
  allowedCapabilities: string[];
  currentAttemptId?: string;
  diagnosticResultId?: string;
  diagnosticRunId?: string;
  diagnosticStage?: string;
  diagnosticSourceKind?: string;
  diagnosticSourceId?: string;
  diagnosticAllowedCapabilities?: string[];
  redactionStatus: SetupRedactionStatus;
  resourceRefs?: SetupResourceRef[];
  redactedEvidence?: Record<string, string>;
  oauthStateRef?: string;
  createdAt: string;
  updatedAt: string;
  lastTransitionAt: string;
  lastTransitionAuditEventId?: string;
  operatorRemediation?: string;
  userRemediation?: string;
  unsupportedReasonCode?: string;
};

export type SetupDiagnosticResource = {
  setupSessionId: string;
  targetId: string;
  diagnosticResultId: string;
  diagnosticRunId?: string;
  diagnosticStage?: string;
  diagnosticSourceKind?: string;
  diagnosticSourceId?: string;
  status: SetupState;
  reasonCode: string;
  retrySafety: SetupRetrySafety;
  remediationOwner: SetupRemediationOwner;
  allowedCapabilities?: string[];
  checkedAt: string;
  staleAfter: string;
  redactionStatus: SetupRedactionStatus;
};

export type SetupTargetListResponse = {
  items: SetupTargetResource[];
};

export type SetupSessionListResponse = {
  items: SetupSessionResource[];
};

export type SetupSessionResponse = {
  session: SetupSessionResource;
};

export type StartSetupInput = {
  targetId: string;
  setupStyle: SetupStyle;
  source?: string;
};

export type SubmitSetupSecretInput = {
  secretRef: string;
  value: string;
  displayName?: string;
};

export type StartSetupOAuthInput = {
  redirectRoute?: string;
};

export type SetupOAuthStartResponse = SetupSessionResponse & {
  authorizationUrl: string;
  state: string;
};

export type CompleteSetupOAuthInput = {
  state: string;
  result: SetupOAuthResult;
  accountLabel?: string;
};

export type DisableSetupInput = {
  disabledReason?: string;
};

export type ReplaceSetupInput = Record<string, never>;

export type SetupDiagnosticListResponse = {
  items: SetupDiagnosticResource[];
};

export type BillingEnforcementMode = "enforced" | "unlimited" | "not_measurable";
export type BillingQuotaStatus = "available" | "near_limit" | "exhausted" | "unlimited" | "not_measurable" | "restricted" | "unavailable";
export type BillingNearLimitReason = "percent_threshold" | "below_one_typical_operation";
export type BillingRecoveryAction = "wait" | "reduce_scope" | "request_override" | "contact_support" | "operator_resolution_required" | "retry_later";
export type BillingDenialClassification = "quota_exhaustion" | "abuse_restriction" | "quota_state_unavailable" | "unauthorized" | "operator_action_needed";

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

export type BillingUsagePeriodSummary = {
  periodStart: string;
  periodEnd: string;
  periodAnchor: string;
  consumedAmount: number;
  reservedAmount: number;
  adjustedAmount: number;
  carryoverApplied: number;
  remainingAmount: number;
  overLimit?: boolean;
};

export type BillingQuotaOverrideSummary = {
  baseLimit: number;
  effectiveLimit: number;
  reason?: string;
  effectiveAt?: string;
  expiresAt?: string;
};

export type BillingAbuseRestrictionSummary = {
  restrictionId?: string;
  status: "active" | "expired" | string;
  affectedCategory?: string;
  recoveryAction?: BillingRecoveryAction | string;
  visibleReasonCode?: string;
  sourceAuditRef?: string;
  supportContactAllowed?: boolean;
  startedAt?: string;
  expiresAt?: string;
};

export type BillingQuotaStatusItem = {
  category: string;
  unit: "count" | "bytes" | "attempts" | string;
  status: BillingQuotaStatus;
  currentPeriod: BillingUsagePeriodSummary;
  previousPeriod?: BillingUsagePeriodSummary;
  limit: number;
  remainingAmount: number;
  nearLimit: boolean;
  nearLimitReason?: BillingNearLimitReason;
  typicalOperationAmount: number;
  baseLimit?: number;
  effectiveLimit?: number;
  override?: BillingQuotaOverrideSummary;
  restriction?: BillingAbuseRestrictionSummary;
  recoveryActions: BillingRecoveryAction[];
};

export type BillingQuotaSection = {
  sectionKey: string;
  label: string;
  items: BillingQuotaStatusItem[];
};

export type BillingQuotaDashboardResponse = {
  tenantId: string;
  plan: {
    planKey: string;
    enforcementMode: BillingEnforcementMode;
    status?: string;
    effectiveAt: string;
    basePlanLabel?: string;
    checkoutAvailable: boolean;
  };
  sections: BillingQuotaSection[];
  generatedAt: string;
  permission?: { allowed: boolean; reasonCode?: string };
};

export type BillingDenialDetailResponse = {
  denialId: string;
  tenantId: string;
  operationRef: string;
  operationKey: string;
  guardedEntryPoint: string;
  category?: string;
  reasonCode: string;
  classification: BillingDenialClassification;
  requestedAmount: number;
  remainingAmount: number;
  recoveryActions: BillingRecoveryAction[];
  restriction?: BillingAbuseRestrictionSummary;
  createdAt: string;
};

export type BillingEvidenceRedaction = {
  path: string;
  reason: string;
  replacement: string;
};

export type BillingEvidenceExportResponse = {
  schemaVersion: string;
  exportId: string;
  tenantId: string;
  generatedAt: string;
  generatedByPrincipalId: string;
  denial: BillingDenialDetailResponse;
  usageSnapshot: BillingQuotaStatusItem[];
  effectiveLimitState: Record<string, unknown>;
  auditRefs: string[];
  redactions: BillingEvidenceRedaction[];
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
  readonly activationFailure?: ActivationFailurePayload;

  constructor(message: string, options: { status: number; code?: string; tenantDenied?: boolean; denial?: TenantDenialResource; quotaDenial?: BillingQuotaDenialPayload; activationFailure?: ActivationFailurePayload }) {
    super(message);
    this.name = "DopeClientError";
    this.status = options.status;
    this.code = options.code;
    this.tenantDenied = options.tenantDenied;
    this.denial = options.denial;
    this.quotaDenial = options.quotaDenial;
    this.activationFailure = options.activationFailure;
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

  async getConfig(tenantOptions?: TenantRequestOptions): Promise<ConfigResponse> {
    return this.requestJSON<ConfigResponse>("/v1/config", { tenant: tenantOptions });
  }

  async getDiscordSetup(connectorId: string, tenantOptions?: TenantRequestOptions): Promise<DiscordHostedSetupResource> {
    return this.requestJSON<DiscordHostedSetupResource>(`/v1/connectors/${encodePathComponent(connectorId.trim())}/discord-setup`, { tenant: tenantOptions });
  }

  async getDiscordSmokeEvidence(connectorId: string, tenantOptions?: TenantRequestOptions): Promise<DiscordSmokeEvidenceResource> {
    return this.requestJSON<DiscordSmokeEvidenceResource>(`/v1/connectors/${encodePathComponent(connectorId.trim())}/discord-smoke`, { tenant: tenantOptions });
  }

  async getDiscordConformanceEvidence(connectorId: string, tenantOptions?: TenantRequestOptions): Promise<DiscordConformanceEvidenceResponse> {
    return this.requestJSON<DiscordConformanceEvidenceResponse>("/v1/live-validations/discord-conformance", {
      query: { connectorId: connectorId.trim() },
      tenant: tenantOptions
    });
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

  async getBillingQuotaDashboard(tenantOptions?: TenantRequestOptions): Promise<BillingQuotaDashboardResponse> {
    return this.requestJSON<BillingQuotaDashboardResponse>("/v1/billing/quota-dashboard", { tenant: tenantOptions });
  }

  async getBillingDenialDetail(denialId: string, tenantOptions?: TenantRequestOptions): Promise<BillingDenialDetailResponse> {
    return this.requestJSON<BillingDenialDetailResponse>(`/v1/billing/denials/${encodePathComponent(denialId.trim())}`, { tenant: tenantOptions });
  }

  async exportBillingDenialEvidence(denialId: string, tenantOptions?: TenantRequestOptions): Promise<BillingEvidenceExportResponse> {
    return this.requestJSON<BillingEvidenceExportResponse>(`/v1/billing/denials/${encodePathComponent(denialId.trim())}/evidence-export`, {
      method: "POST",
      tenant: tenantOptions
    });
  }

  async getActivation(tenantOptions?: TenantRequestOptions): Promise<ActivationResponse> {
    return this.requestJSON<ActivationResponse>("/v1/activation", { tenant: tenantOptions });
  }

  async activate(input: RunActivationInput = {}, tenantOptions?: TenantRequestOptions): Promise<ActivationResponse> {
    return this.requestJSON<ActivationResponse>("/v1/activation", {
      method: "POST",
      body: normalizeActivationInput(input),
      tenant: tenantOptions
    });
  }

  async runActivationTestChat(input: RunActivationTestChatInput, tenantOptions?: TenantRequestOptions): Promise<ActivationTestChatResponse> {
    return this.requestJSON<ActivationTestChatResponse>("/v1/activation/test-chat", {
      method: "POST",
      body: normalizeActivationTestChatInput(input),
      tenant: tenantOptions
    });
  }

  async getActivationDiagnostics(tenantOptions?: TenantRequestOptions): Promise<ActivationDiagnosticListResponse> {
    return this.requestJSON<ActivationDiagnosticListResponse>("/v1/activation/diagnostics", { tenant: tenantOptions });
  }

  async listSetupTargets(tenantOptions?: TenantRequestOptions): Promise<SetupTargetListResponse> {
    return this.requestJSON<SetupTargetListResponse>("/v1/setup/targets", { tenant: tenantOptions });
  }

  async listSetupSessions(tenantOptions?: TenantRequestOptions): Promise<SetupSessionListResponse> {
    return this.requestJSON<SetupSessionListResponse>("/v1/setup/sessions", { tenant: tenantOptions });
  }

  async startSetup(input: StartSetupInput, tenantOptions?: TenantRequestOptions): Promise<SetupSessionResponse> {
    return this.requestJSON<SetupSessionResponse>("/v1/setup/sessions", {
      method: "POST",
      body: normalizeStartSetupInput(input),
      tenant: tenantOptions
    });
  }

  async getSetupSession(setupSessionId: string, tenantOptions?: TenantRequestOptions): Promise<SetupSessionResponse> {
    return this.requestJSON<SetupSessionResponse>(`/v1/setup/sessions/${encodePathComponent(setupSessionId)}`, { tenant: tenantOptions });
  }

  async submitSetupSecret(setupSessionId: string, input: SubmitSetupSecretInput, tenantOptions?: TenantRequestOptions): Promise<SetupSessionResponse> {
    return this.requestJSON<SetupSessionResponse>(`/v1/setup/sessions/${encodePathComponent(setupSessionId)}/submit-secret`, {
      method: "POST",
      body: {
        secretRef: input.secretRef.trim(),
        value: input.value,
        displayName: input.displayName?.trim() || undefined
      },
      tenant: tenantOptions
    });
  }

  async startSetupOAuth(setupSessionId: string, input: StartSetupOAuthInput = {}, tenantOptions?: TenantRequestOptions): Promise<SetupOAuthStartResponse> {
    return this.requestJSON<SetupOAuthStartResponse>(`/v1/setup/sessions/${encodePathComponent(setupSessionId)}/oauth/start`, {
      method: "POST",
      body: { redirectRoute: input.redirectRoute?.trim() || undefined },
      tenant: tenantOptions
    });
  }

  async completeSetupOAuth(setupSessionId: string, input: CompleteSetupOAuthInput, tenantOptions?: TenantRequestOptions): Promise<SetupSessionResponse> {
    return this.requestJSON<SetupSessionResponse>(`/v1/setup/sessions/${encodePathComponent(setupSessionId)}/oauth/callback`, {
      method: "POST",
      body: {
        state: input.state.trim(),
        result: input.result,
        accountLabel: input.accountLabel?.trim() || undefined
      },
      tenant: tenantOptions
    });
  }

  async retrySetup(setupSessionId: string, tenantOptions?: TenantRequestOptions): Promise<SetupSessionResponse> {
    return this.requestJSON<SetupSessionResponse>(`/v1/setup/sessions/${encodePathComponent(setupSessionId)}/retry`, {
      method: "POST",
      tenant: tenantOptions
    });
  }

  async replaceSetup(setupSessionId: string, inputOrTenantOptions?: ReplaceSetupInput | TenantRequestOptions, tenantOptions?: TenantRequestOptions): Promise<SetupSessionResponse> {
    const tenant = tenantOptions ?? (isTenantRequestOptions(inputOrTenantOptions) ? inputOrTenantOptions : undefined);
    return this.requestJSON<SetupSessionResponse>(`/v1/setup/sessions/${encodePathComponent(setupSessionId)}/replace`, {
      method: "POST",
      tenant
    });
  }

  async cancelSetup(setupSessionId: string, tenantOptions?: TenantRequestOptions): Promise<SetupSessionResponse> {
    return this.requestJSON<SetupSessionResponse>(`/v1/setup/sessions/${encodePathComponent(setupSessionId)}/cancel`, {
      method: "POST",
      tenant: tenantOptions
    });
  }

  async disableSetup(setupSessionId: string, input: DisableSetupInput = {}, tenantOptions?: TenantRequestOptions): Promise<SetupSessionResponse> {
    return this.requestJSON<SetupSessionResponse>(`/v1/setup/sessions/${encodePathComponent(setupSessionId)}/disable`, {
      method: "POST",
      body: { disabledReason: input.disabledReason?.trim() || undefined },
      tenant: tenantOptions
    });
  }

  async getSetupDiagnostics(setupSessionId: string, tenantOptions?: TenantRequestOptions): Promise<SetupDiagnosticListResponse> {
    return this.requestJSON<SetupDiagnosticListResponse>(`/v1/setup/sessions/${encodePathComponent(setupSessionId)}/diagnostics`, {
      tenant: tenantOptions
    });
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

  async listEvaluationDiscoveryPolicies(query: EvaluationDiscoveryPolicyQuery = {}, tenantOptions?: TenantRequestOptions): Promise<EvaluationProductListResponse<EvaluationDiscoveryPolicyResource>> {
    return this.requestJSON<EvaluationProductListResponse<EvaluationDiscoveryPolicyResource>>("/v1/evaluation/discovery-policies", { query, tenant: tenantOptions });
  }

  async getEvaluationDiscoveryPolicy(policyId: string, tenantOptions?: TenantRequestOptions): Promise<EvaluationDiscoveryPolicyResource> {
    return this.requestJSON<EvaluationDiscoveryPolicyResource>(`/v1/evaluation/discovery-policies/${policyId.trim()}`, { tenant: tenantOptions });
  }

  async upsertEvaluationDiscoveryPolicy(policyId: string, input: UpsertEvaluationDiscoveryPolicyInput, tenantOptions?: TenantRequestOptions): Promise<EvaluationDiscoveryPolicyResource> {
    return this.requestJSON<EvaluationDiscoveryPolicyResource>(`/v1/evaluation/discovery-policies/${policyId.trim()}`, {
      method: "PUT",
      body: input,
      tenant: tenantOptions
    });
  }

  async startEvaluationDiscoveryRun(input: StartEvaluationDiscoveryRunInput = {}, tenantOptions?: TenantRequestOptions): Promise<EvaluationDiscoveryRunResource> {
    return this.requestJSON<EvaluationDiscoveryRunResource>("/v1/evaluation/discovery-runs", {
      method: "POST",
      body: input,
      tenant: tenantOptions
    });
  }

  async listEvaluationDiscoveryRuns(query: EvaluationDiscoveryRunQuery = {}, tenantOptions?: TenantRequestOptions): Promise<EvaluationProductListResponse<EvaluationDiscoveryRunResource>> {
    return this.requestJSON<EvaluationProductListResponse<EvaluationDiscoveryRunResource>>("/v1/evaluation/discovery-runs", { query, tenant: tenantOptions });
  }

  async getEvaluationDiscoveryRun(discoveryRunId: string, tenantOptions?: TenantRequestOptions): Promise<EvaluationDiscoveryRunResource> {
    return this.requestJSON<EvaluationDiscoveryRunResource>(`/v1/evaluation/discovery-runs/${discoveryRunId.trim()}`, { tenant: tenantOptions });
  }

  async listEvaluationDiscoveredCandidates(query: EvaluationDiscoveredCandidateQuery = {}, tenantOptions?: TenantRequestOptions): Promise<EvaluationProductListResponse<EvaluationDiscoveredCandidateResource>> {
    return this.requestJSON<EvaluationProductListResponse<EvaluationDiscoveredCandidateResource>>("/v1/evaluation/discovered-candidates", { query, tenant: tenantOptions });
  }

  async getEvaluationDiscoveredCandidate(discoveredCandidateId: string, tenantOptions?: TenantRequestOptions): Promise<EvaluationDiscoveredCandidateResource> {
    return this.requestJSON<EvaluationDiscoveredCandidateResource>(`/v1/evaluation/discovered-candidates/${discoveredCandidateId.trim()}`, { tenant: tenantOptions });
  }

  async createEvaluationSuppression(input: CreateEvaluationSuppressionInput, tenantOptions?: TenantRequestOptions): Promise<EvaluationSuppressionRecord> {
    return this.requestJSON<EvaluationSuppressionRecord>("/v1/evaluation/suppressions", {
      method: "POST",
      body: input,
      tenant: tenantOptions
    });
  }

  async materializeProductFixture(discoveredCandidateId: string, input: CreateProductFixtureInput, tenantOptions?: TenantRequestOptions): Promise<ProductFixtureMutationResponse> {
    return this.requestJSON<ProductFixtureMutationResponse>(`/v1/evaluation/discovered-candidates/${discoveredCandidateId.trim()}/product-fixtures`, {
      method: "POST",
      body: input,
      tenant: tenantOptions
    });
  }

  async listProductFixtures(query: ProductFixtureQuery = {}, tenantOptions?: TenantRequestOptions): Promise<EvaluationProductListResponse<ProductFixtureResource>> {
    return this.requestJSON<EvaluationProductListResponse<ProductFixtureResource>>("/v1/evaluation/product-fixtures", { query, tenant: tenantOptions });
  }

  async getProductFixture(fixtureId: string, tenantOptions?: TenantRequestOptions): Promise<ProductFixtureResource> {
    return this.requestJSON<ProductFixtureResource>(`/v1/evaluation/product-fixtures/${fixtureId.trim()}`, { tenant: tenantOptions });
  }

  async listProductFixtureRevisions(fixtureId: string, query: EvaluationProductTenantOptions = {}, tenantOptions?: TenantRequestOptions): Promise<EvaluationProductListResponse<FixtureRevisionResource>> {
    return this.requestJSON<EvaluationProductListResponse<FixtureRevisionResource>>(`/v1/evaluation/product-fixtures/${fixtureId.trim()}/revisions`, { query, tenant: tenantOptions });
  }

  async createProductFixtureRevision(fixtureId: string, input: CreateFixtureRevisionInput, tenantOptions?: TenantRequestOptions): Promise<ProductFixtureMutationResponse> {
    return this.requestJSON<ProductFixtureMutationResponse>(`/v1/evaluation/product-fixtures/${fixtureId.trim()}/revisions`, {
      method: "POST",
      body: input,
      tenant: tenantOptions
    });
  }

  async reviewProductFixture(fixtureId: string, input: ReviewProductFixtureInput, tenantOptions?: TenantRequestOptions): Promise<ProductFixtureMutationResponse> {
    return this.requestJSON<ProductFixtureMutationResponse>(`/v1/evaluation/product-fixtures/${fixtureId.trim()}/review`, {
      method: "POST",
      body: input,
      tenant: tenantOptions
    });
  }

  async suppressProductFixture(fixtureId: string, input: Partial<Pick<CreateEvaluationSuppressionInput, "reasonCode" | "reason" | "expiresAt">> = {}, tenantOptions?: TenantRequestOptions): Promise<ProductFixtureMutationResponse> {
    return this.requestJSON<ProductFixtureMutationResponse>(`/v1/evaluation/product-fixtures/${fixtureId.trim()}/suppress`, {
      method: "POST",
      body: input,
      tenant: tenantOptions
    });
  }

  async createEvaluationCampaign(input: CreateEvaluationCampaignInput, tenantOptions?: TenantRequestOptions): Promise<EvaluationCampaignResource> {
    return this.requestJSON<EvaluationCampaignResource>("/v1/evaluation/campaigns", {
      method: "POST",
      body: input,
      tenant: tenantOptions
    });
  }

  async listEvaluationCampaigns(query: EvaluationProductTenantOptions = {}, tenantOptions?: TenantRequestOptions): Promise<EvaluationProductListResponse<EvaluationCampaignResource>> {
    return this.requestJSON<EvaluationProductListResponse<EvaluationCampaignResource>>("/v1/evaluation/campaigns", { query, tenant: tenantOptions });
  }

  async getEvaluationCampaign(campaignId: string, tenantOptions?: TenantRequestOptions): Promise<EvaluationCampaignResource> {
    return this.requestJSON<EvaluationCampaignResource>(`/v1/evaluation/campaigns/${campaignId.trim()}`, { tenant: tenantOptions });
  }

  async startEvaluationCampaign(campaignId: string, tenantOptions?: TenantRequestOptions): Promise<EvaluationCampaignResource> {
    return this.requestJSON<EvaluationCampaignResource>(`/v1/evaluation/campaigns/${campaignId.trim()}/start`, { method: "POST", tenant: tenantOptions });
  }

  async cancelEvaluationCampaign(campaignId: string, tenantOptions?: TenantRequestOptions): Promise<EvaluationCampaignResource> {
    return this.requestJSON<EvaluationCampaignResource>(`/v1/evaluation/campaigns/${campaignId.trim()}/cancel`, { method: "POST", tenant: tenantOptions });
  }

  async publishEvaluationCampaignResults(campaignId: string, tenantOptions?: TenantRequestOptions): Promise<EvaluationCampaignResource> {
    return this.requestJSON<EvaluationCampaignResource>(`/v1/evaluation/campaigns/${campaignId.trim()}/publish-results`, { method: "POST", tenant: tenantOptions });
  }

  async listEvaluationCampaignItems(campaignId: string, query: EvaluationProductTenantOptions = {}, tenantOptions?: TenantRequestOptions): Promise<EvaluationProductListResponse<EvaluationCampaignItemResource>> {
    return this.requestJSON<EvaluationProductListResponse<EvaluationCampaignItemResource>>(`/v1/evaluation/campaigns/${campaignId.trim()}/items`, { query, tenant: tenantOptions });
  }

  async listEvaluationCampaignAttemptGroups(campaignId: string, query: EvaluationProductTenantOptions = {}, tenantOptions?: TenantRequestOptions): Promise<EvaluationProductListResponse<EvaluationCampaignAttemptGroupResource>> {
    return this.requestJSON<EvaluationProductListResponse<EvaluationCampaignAttemptGroupResource>>(`/v1/evaluation/campaigns/${campaignId.trim()}/attempt-groups`, { query, tenant: tenantOptions });
  }

  async listEvaluationDashboard(query: EvaluationProductTenantOptions = {}, tenantOptions?: TenantRequestOptions): Promise<EvaluationProductListResponse<EvaluationDashboardProjectionResource>> {
    return this.requestJSON<EvaluationProductListResponse<EvaluationDashboardProjectionResource>>("/v1/evaluation/dashboard", { query, tenant: tenantOptions });
  }

  async listEvaluationToolCallInspections(campaignId: string, query: EvaluationProductTenantOptions = {}, tenantOptions?: TenantRequestOptions): Promise<EvaluationProductListResponse<EvaluationToolCallInspectionResource>> {
    return this.requestJSON<EvaluationProductListResponse<EvaluationToolCallInspectionResource>>(`/v1/evaluation/campaigns/${campaignId.trim()}/tool-call-inspections`, { query, tenant: tenantOptions });
  }

  async getEvaluationToolCallInspection(inspectionId: string, tenantOptions?: TenantRequestOptions): Promise<EvaluationToolCallInspectionResource> {
    return this.requestJSON<EvaluationToolCallInspectionResource>(`/v1/evaluation/tool-call-inspections/${inspectionId.trim()}`, { tenant: tenantOptions });
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

  async listIntegrationDiagnostics(integrationId: string, query: { capability?: string; includeStale?: boolean; limit?: number; cursor?: string } = {}, tenantOptions?: TenantRequestOptions): Promise<IntegrationDiagnosticListResponse> {
    return this.requestJSON<IntegrationDiagnosticListResponse>(`/v1/integrations/${integrationId.trim()}/diagnostics`, { query, tenant: tenantOptions });
  }

  async startIntegrationDiagnosticRun(integrationId: string, input: CreateIntegrationDiagnosticRunInput, tenantOptions?: TenantRequestOptions): Promise<IntegrationDiagnosticRunResource> {
    return this.requestJSON<IntegrationDiagnosticRunResource>(`/v1/integrations/${integrationId.trim()}/diagnostics/runs`, {
      method: "POST",
      body: input,
      tenant: tenantOptions
    });
  }

  async listIntegrationDiagnosticRuns(query: { integrationId?: string; providerKind?: string; domainKind?: string; status?: string; reasonCode?: IntegrationDiagnosticReasonCode; limit?: number; cursor?: string } = {}, tenantOptions?: TenantRequestOptions): Promise<{ items: IntegrationDiagnosticRunResource[]; nextCursor?: string }> {
    return this.requestJSON<{ items: IntegrationDiagnosticRunResource[]; nextCursor?: string }>("/v1/integration-diagnostics/runs", { query, tenant: tenantOptions });
  }

  async getIntegrationDiagnosticRun(runId: string, tenantOptions?: TenantRequestOptions): Promise<IntegrationDiagnosticRunResource> {
    return this.requestJSON<IntegrationDiagnosticRunResource>(`/v1/integration-diagnostics/runs/${runId.trim()}`, { tenant: tenantOptions });
  }

  async createIntegrationDiagnosticSmoke(input: CreateIntegrationDiagnosticSmokeInput, tenantOptions?: TenantRequestOptions): Promise<SmokeMatrixReportResource> {
    return this.requestJSON<SmokeMatrixReportResource>("/v1/integration-diagnostics/smoke", {
      method: "POST",
      body: input,
      tenant: tenantOptions
    });
  }

  async applyIntegrationDiagnosticRetention(query: { limit?: number } = {}, tenantOptions?: TenantRequestOptions): Promise<{ items: DiagnosticRetentionRecordResource[] }> {
    return this.requestJSON<{ items: DiagnosticRetentionRecordResource[] }>("/v1/integration-diagnostics/retention/apply", {
      method: "POST",
      query,
      tenant: tenantOptions
    });
  }

  async listIntegrationDiagnosticReasonCodes(tenantOptions?: TenantRequestOptions): Promise<{ items: IntegrationDiagnosticReasonCodeResource[] }> {
    return this.requestJSON<{ items: IntegrationDiagnosticReasonCodeResource[] }>("/v1/integration-diagnostics/reason-codes", { tenant: tenantOptions });
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

function normalizeActivationInput(input: RunActivationInput): RunActivationInput {
  return {
    source: input.source?.trim() || undefined
  };
}

function normalizeActivationTestChatInput(input: RunActivationTestChatInput): RunActivationTestChatInput {
  return {
    message: input.message.trim()
  };
}

function normalizeStartSetupInput(input: StartSetupInput): StartSetupInput {
  return {
    targetId: input.targetId.trim(),
    setupStyle: input.setupStyle,
    source: input.source?.trim() || undefined
  };
}

async function toClientError(response: Response): Promise<DopeClientError> {
  let message = `request failed with status ${response.status}`;
  let code: string | undefined;
  let denial: TenantDenialResource | undefined;
  let quotaDenial: BillingQuotaDenialPayload | undefined;
  let activationFailure: ActivationFailurePayload | undefined;

  try {
    const payload = (await response.json()) as {
      error?: string;
      errorCode?: string;
      requestId?: string;
      code?: string;
      reasonCode?: string;
      stage?: string;
      retryable?: boolean;
      remediationOwner?: string;
    } & Partial<BillingQuotaDenialPayload>;
    if (payload.error) {
      message = payload.error;
    }
    if (payload.errorCode || payload.reasonCode || payload.code) {
      code = payload.errorCode ?? payload.reasonCode ?? payload.code;
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
    if (payload.reasonCode?.startsWith("activation_") && payload.stage && typeof payload.retryable === "boolean" && payload.remediationOwner) {
      activationFailure = {
        code: payload.code as ActivationReasonCode | undefined,
        reasonCode: payload.reasonCode as ActivationReasonCode,
        stage: payload.stage,
        retryable: payload.retryable,
        remediationOwner: payload.remediationOwner as ActivationRemediationOwner
      };
      code = payload.reasonCode;
    }
  } catch {
    // Ignore non-json failure bodies.
  }

  const tenantDenied = Boolean(denial) || isTenantDenialCode(code);
  return new DopeClientError(message, { status: response.status, code, tenantDenied: tenantDenied || undefined, denial, quotaDenial, activationFailure });
}

function isTenantDenialCode(code: string | undefined): boolean {
  return code === "tenant_access_denied" || code === "tenant_permission_denied" || code === "tenant_ownership_denied" || code?.startsWith("credential_denied:") === true || code?.startsWith("setup_denied:") === true;
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
