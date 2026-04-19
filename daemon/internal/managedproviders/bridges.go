package managedproviders

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/llm"
	"github.com/dopejs/dope-agent/daemon/internal/providers"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
)

const (
	managedProviderMetadataProviderID      = "managedProviderId"
	managedProviderMetadataAction          = "managedProviderAction"
	managedProviderMetadataOperationID     = "managedProviderOperationId"
	managedProviderMetadataProfileID       = "sandboxProfileId"
	managedProviderMetadataDecision        = "sandboxDecision"
	managedProviderMetadataFailureClass    = "failureClass"
	managedProviderMetadataStrength        = "enforcementStrength"
	managedProviderMetadataSensitiveStates = "sensitiveStateClasses"
	managedProviderMetadataAccessSummary   = "localStateAccesses"
	managedProviderRequestedByPrefix       = "managed_provider:"
	managedProviderRedactionRule           = "class_summary_only"
)

type managedProviderOperationContextKey struct{}

type managedProviderOperationPlan struct {
	OperationID    string
	ProviderID     string
	Action         sandbox.ManagedProviderActionKind
	ProfileID      string
	RequestedBy    string
	Reason         string
	DeclaredRead   []string
	DeclaredWrite  []string
	Access         sandbox.AccessRequest
	LocalState     []sandbox.SensitiveLocalStateAccessSummary
	SensitiveKinds []string
}

type managedProviderOperationEvaluation struct {
	Declaration sandbox.ManagedProviderRequirementDeclaration
	Operation   sandbox.ManagedProviderOperation
	Metadata    map[string]string
	Consumer    *sandbox.ConsumerContractView
}

type Bridge interface {
	ProviderID() string
	DisplayName() string
	Family() providers.Family
	AuthMode() providers.AuthMode
	Detect(ctx context.Context) (providers.AuthState, []providers.Model, error)
	Start(ctx context.Context) (providers.AuthState, []providers.Model, error)
	Complete(ctx context.Context) (providers.AuthState, []providers.Model, error)
	Refresh(ctx context.Context) (providers.AuthState, []providers.Model, error)
	Revoke(ctx context.Context) (providers.AuthState, []providers.Model, error)
	Provider() llm.Provider
}

type Runner interface {
	Run(ctx context.Context, cmd string, args []string, workdir string) (RunResult, error)
}

type RunResult struct {
	ExecutionID string
	Stdout      string
	Stderr      string
	ExitCode    int
}

type RunError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e *RunError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	return e.Code
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, cmd string, args []string, workdir string) (RunResult, error) {
	command := exec.CommandContext(ctx, cmd, args...)
	if workdir != "" {
		command.Dir = workdir
	}
	output, err := command.CombinedOutput()
	result := RunResult{
		Stdout: strings.TrimSpace(string(output)),
		Stderr: strings.TrimSpace(string(output)),
	}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, err
	}
	return result, err
}

type sandboxRunner struct {
	manager    *sandbox.Manager
	profileID  string
	providerID string
	roots      []string
}

func (r sandboxRunner) Run(ctx context.Context, cmd string, args []string, workdir string) (RunResult, error) {
	if r.manager == nil {
		return execRunner{}.Run(ctx, cmd, args, workdir)
	}
	requestedBy := managedProviderRequestedByPrefix + r.providerID
	access := sandbox.AccessRequest{
		ReadRoots:     cloneRoots(r.roots),
		WriteRoots:    cloneRoots(r.roots),
		NetworkMode:   sandbox.NetworkModeFull,
		AllowedHosts:  []string{},
		AllowedPorts:  []int{},
		AllowLoopback: true,
	}
	metadata := map[string]string{}
	var consumer *sandbox.ConsumerContractView
	if operation, ok := managedProviderOperationFromContext(ctx); ok {
		requestedBy = firstNonEmpty(strings.TrimSpace(operation.RequestedBy), requestedBy)
		access = cloneAccessRequest(operation.Access)
		metadata = cloneStringMap(operationMetadataFromPlan(operation))
		consumer = buildManagedProviderConsumerView(operation, nil)
	}
	request := sandbox.ExecutionRequest{
		ProfileID:    r.profileID,
		Command:      cmd,
		Args:         append([]string(nil), args...),
		Cwd:          workdir,
		RequestedBy:  requestedBy,
		ResourceKind: "provider",
		ResourceID:   r.providerID,
		Scope:        "managed_provider",
		Reason:       "managed provider bridge execution",
		Metadata:     metadata,
		Access:       access,
		Consumer:     consumer,
	}
	execution, err := r.manager.StartExecution(ctx, request)
	if err != nil {
		return RunResult{}, err
	}
	execution, err = r.manager.WaitExecution(ctx, execution.ExecutionID)
	if err != nil {
		return RunResult{}, err
	}
	result := RunResult{
		ExecutionID: execution.ExecutionID,
		Stdout:      strings.TrimSpace(execution.Result.Stdout),
		Stderr:      strings.TrimSpace(execution.Result.Stderr),
	}
	if execution.Result.ExitCode != nil {
		result.ExitCode = *execution.Result.ExitCode
	}
	switch execution.Status {
	case sandbox.ExecutionStatusCompleted:
		return result, nil
	case sandbox.ExecutionStatusFailed:
		if execution.Result.ErrorClass == sandbox.ErrorClassProcessFailed {
			return result, errors.New(firstNonEmpty(execution.Result.Stderr, execution.Result.Stdout, execution.Result.Error, "sandbox process failed"))
		}
		return result, &RunError{
			Code:      firstNonEmpty(execution.Result.ErrorCode, "sandbox_execution_failed"),
			Message:   firstNonEmpty(execution.Result.Error, execution.Result.Stderr, execution.Result.Stdout, "sandbox execution failed"),
			Retryable: execution.Result.ErrorClass == sandbox.ErrorClassTimeout,
		}
	case sandbox.ExecutionStatusCancelled:
		return result, &RunError{
			Code:      firstNonEmpty(execution.Result.ErrorCode, "sandbox_cancelled"),
			Message:   firstNonEmpty(execution.Result.Error, "sandbox execution was cancelled"),
			Retryable: false,
		}
	case sandbox.ExecutionStatusDenied:
		return result, &RunError{
			Code:      firstNonEmpty(execution.Result.ErrorCode, "sandbox_policy_denied"),
			Message:   firstNonEmpty(execution.Result.Error, "sandbox execution was denied"),
			Retryable: false,
		}
	default:
		return result, &RunError{
			Code:      "sandbox_unknown_status",
			Message:   "sandbox execution returned unexpected status",
			Retryable: false,
		}
	}
}

type Registry struct {
	bridges map[string]providers.ManagedBridge
	order   []string
}

func NewRegistry(cfg config.Config, sandboxes *sandbox.Manager) *Registry {
	homeDir := strings.TrimSpace(config.ManagedProviderHomeDir(cfg))
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	registry := &Registry{
		bridges: make(map[string]providers.ManagedBridge),
	}
	if homeDir != "" {
		_ = os.MkdirAll(homeDir, 0o755)
	}

	claudeWorkDir := firstNonEmpty(resolvePath(homeDir, cfg.LLM.Claude.WorkDir), homeFallbackWorkdir(homeDir))
	codexWorkDir := firstNonEmpty(resolvePath(homeDir, cfg.LLM.Codex.WorkDir), homeFallbackWorkdir(homeDir))

	claudeRunner := Runner(execRunner{})
	codexRunner := Runner(execRunner{})
	if sandboxes != nil {
		claudeRunner = sandboxRunner{
			manager:    sandboxes,
			profileID:  sandbox.ProfileIDManagedProviderClaude,
			providerID: ClaudeProviderID,
			roots:      []string{claudeWorkDir, filepath.Join(homeDir, ".claude"), os.TempDir()},
		}
		codexRunner = sandboxRunner{
			manager:    sandboxes,
			profileID:  sandbox.ProfileIDManagedProviderCodex,
			providerID: CodexProviderID,
			roots:      []string{codexWorkDir, filepath.Join(homeDir, ".codex"), os.TempDir()},
		}
	}

	items := []providers.ManagedBridge{
		newClaudeBridge(homeDir, cfg, claudeRunner, sandboxes),
		newCodexBridge(homeDir, cfg, codexRunner, sandboxes),
	}
	for _, bridge := range items {
		registry.bridges[bridge.ProviderID()] = bridge
		registry.order = append(registry.order, bridge.ProviderID())
	}
	return registry
}

func (r *Registry) List() []providers.ManagedBridge {
	items := make([]providers.ManagedBridge, 0, len(r.order))
	for _, id := range r.order {
		items = append(items, r.bridges[id])
	}
	return items
}

func (r *Registry) Get(providerID string) (providers.ManagedBridge, bool) {
	item, ok := r.bridges[strings.TrimSpace(providerID)]
	return item, ok
}

func firstAvailablePath(explicit string, candidates ...string) string {
	if trimmed := strings.TrimSpace(explicit); trimmed != "" {
		return trimmed
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path
		}
	}
	return ""
}

func decodeJWTPayload(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil
	}
	raw := parts[1]
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}
	return claims
}

func homeFallbackWorkdir(homeDir string) string {
	if strings.TrimSpace(homeDir) == "" {
		return "."
	}
	return homeDir
}

func nowPtr(t time.Time) *time.Time {
	return &t
}

func resolvePath(homeDir, value string) string {
	trimmed := strings.TrimSpace(value)
	switch {
	case trimmed == "":
		return ""
	case trimmed == "~":
		return homeDir
	case strings.HasPrefix(trimmed, "~/"):
		return filepath.Join(homeDir, strings.TrimPrefix(trimmed, "~/"))
	default:
		return trimmed
	}
}

func cloneRoots(values []string) []string {
	items := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	return items
}

func cloneAccessRequest(access sandbox.AccessRequest) sandbox.AccessRequest {
	return sandbox.AccessRequest{
		ReadRoots:     cloneRoots(access.ReadRoots),
		WriteRoots:    cloneRoots(access.WriteRoots),
		NetworkMode:   access.NetworkMode,
		AllowedHosts:  append([]string(nil), access.AllowedHosts...),
		AllowedPorts:  append([]int(nil), access.AllowedPorts...),
		AllowLoopback: access.AllowLoopback,
	}
}

func managedProviderOperationFromContext(ctx context.Context) (managedProviderOperationPlan, bool) {
	if ctx == nil {
		return managedProviderOperationPlan{}, false
	}
	value, ok := ctx.Value(managedProviderOperationContextKey{}).(managedProviderOperationPlan)
	return value, ok
}

func withManagedProviderOperation(ctx context.Context, operation managedProviderOperationPlan) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	operation.Access = cloneAccessRequest(operation.Access)
	operation.LocalState = cloneLocalStateSummaries(operation.LocalState)
	operation.SensitiveKinds = cloneStrings(operation.SensitiveKinds)
	return context.WithValue(ctx, managedProviderOperationContextKey{}, operation)
}

func evaluateManagedProviderOperation(manager *sandbox.Manager, operation managedProviderOperationPlan) (managedProviderOperationEvaluation, error) {
	now := time.Now().UTC()
	evaluation := managedProviderOperationEvaluation{
		Declaration: sandbox.ManagedProviderRequirementDeclaration{
			ProviderID:            strings.TrimSpace(operation.ProviderID),
			ActionKind:            operation.Action,
			ProfileID:             strings.TrimSpace(operation.ProfileID),
			BackendKind:           sandbox.BackendKindSubprocess,
			ReadRoots:             cloneRoots(firstNonEmptyRoots(operation.DeclaredRead, operation.Access.ReadRoots)),
			WriteRoots:            cloneRoots(firstNonEmptyRoots(operation.DeclaredWrite, operation.Access.WriteRoots)),
			NetworkMode:           operation.Access.NetworkMode,
			AllowedHosts:          cloneStrings(operation.Access.AllowedHosts),
			AllowedPorts:          cloneInts(operation.Access.AllowedPorts),
			ApprovalMode:          sandbox.ApprovalModeAllow,
			SensitiveStateClasses: cloneStrings(operation.SensitiveKinds),
			EnforcementStrength:   "declared_only",
			Active:                true,
		},
		Operation: sandbox.ManagedProviderOperation{
			OperationID:               newManagedProviderOperationID(),
			ProviderID:                strings.TrimSpace(operation.ProviderID),
			ActionKind:                operation.Action,
			RequestedBy:               firstNonEmpty(strings.TrimSpace(operation.RequestedBy), managedProviderRequestedByPrefix+strings.TrimSpace(operation.ProviderID)),
			RequirementProfileID:      strings.TrimSpace(operation.ProfileID),
			Decision:                  sandbox.DecisionResolutionAllow,
			ApprovalStatus:            sandbox.DecisionApprovalStatusNotApplicable,
			EnforcementStrength:       "declared_only",
			SensitiveStateClasses:     cloneStrings(operation.SensitiveKinds),
			StartedAt:                 now,
			Status:                    sandbox.ManagedProviderOperationStatusLocalStateInspection,
			LocalStateAccessSummaries: cloneLocalStateSummaries(operation.LocalState),
		},
	}
	evaluation.Consumer = buildManagedProviderConsumerView(operation, &evaluation)

	if manager != nil {
		decision, err := manager.EvaluateAccess(operation.ProfileID, "", operation.Access)
		if err != nil {
			return managedProviderOperationEvaluation{}, err
		}
		evaluation.Operation.Decision = decision.Resolution
		evaluation.Operation.ApprovalStatus = decision.ApprovalStatus
		if decision.Resolution == sandbox.DecisionResolutionDeny {
			evaluation.Operation.Status = sandbox.ManagedProviderOperationStatusDenied
			evaluation.Operation.FailureClass = string(sandbox.ErrorClassPolicyDenied)
		}
		if decision.Resolution == sandbox.DecisionResolutionAsk {
			evaluation.Operation.Status = sandbox.ManagedProviderOperationStatusDenied
			evaluation.Operation.FailureClass = string(sandbox.ErrorClassApprovalRequired)
		}
		if profile, ok := manager.GetProfile(operation.ProfileID); ok {
			evaluation.Declaration.BackendKind = profile.BackendKind
			evaluation.Declaration.ApprovalMode = profile.ApprovalPolicy.Mode
			evaluation.Declaration.EnforcementStrength = firstNonEmpty(profile.NetworkPolicy.EnforcementMode, "declared_only")
			evaluation.Operation.EnforcementStrength = evaluation.Declaration.EnforcementStrength
		}
	}
	if evaluation.Operation.Decision == sandbox.DecisionResolutionAllow {
		if !pathsWithinDeclared(operation.Access.ReadRoots, evaluation.Declaration.ReadRoots) || !pathsWithinDeclared(operation.Access.WriteRoots, evaluation.Declaration.WriteRoots) {
			evaluation.Operation.Decision = sandbox.DecisionResolutionDeny
			evaluation.Operation.Status = sandbox.ManagedProviderOperationStatusDenied
			evaluation.Operation.FailureClass = string(sandbox.ErrorClassPolicyDenied)
		}
	}

	evaluation.Metadata = operationMetadata(evaluation.Operation)
	if evaluation.Consumer != nil {
		evaluation.Consumer.PolicyRecord.Decision = evaluation.Operation.Decision
		evaluation.Consumer.PolicyRecord.ApprovalStatus = evaluation.Operation.ApprovalStatus
		if evaluation.Operation.Status == sandbox.ManagedProviderOperationStatusDenied {
			evaluation.Consumer.PolicyRecord.Status = sandbox.PolicyRecordStatusDenied
			evaluation.Consumer.PolicyRecord.FailureClass = evaluation.Operation.FailureClass
			now := time.Now().UTC()
			evaluation.Consumer.PolicyRecord.CompletedAt = &now
		}
	}
	if manager != nil && evaluation.Consumer != nil {
		_ = manager.PersistConsumerView(context.Background(), evaluation.Consumer)
	}
	return evaluation, nil
}

func operationMetadata(operation sandbox.ManagedProviderOperation) map[string]string {
	metadata := map[string]string{
		managedProviderMetadataProviderID:  strings.TrimSpace(operation.ProviderID),
		managedProviderMetadataAction:      string(operation.ActionKind),
		managedProviderMetadataOperationID: strings.TrimSpace(operation.OperationID),
		managedProviderMetadataProfileID:   strings.TrimSpace(operation.RequirementProfileID),
		managedProviderMetadataDecision:    string(operation.Decision),
		managedProviderMetadataStrength:    strings.TrimSpace(operation.EnforcementStrength),
	}
	if len(operation.SensitiveStateClasses) > 0 {
		metadata[managedProviderMetadataSensitiveStates] = strings.Join(operation.SensitiveStateClasses, ",")
	}
	if operation.FailureClass != "" {
		metadata[managedProviderMetadataFailureClass] = operation.FailureClass
	}
	if encoded, err := json.Marshal(operation.LocalStateAccessSummaries); err == nil && len(operation.LocalStateAccessSummaries) > 0 {
		metadata[managedProviderMetadataAccessSummary] = string(encoded)
	}
	return metadata
}

func buildManagedProviderConsumerView(operation managedProviderOperationPlan, evaluation *managedProviderOperationEvaluation) *sandbox.ConsumerContractView {
	consumerID := strings.TrimSpace(operation.ProviderID)
	operationKind := string(operation.Action)
	declarationID := "managed_provider:" + consumerID + ":" + operationKind
	readRoots := cloneRoots(firstNonEmptyRoots(operation.DeclaredRead, operation.Access.ReadRoots))
	writeRoots := cloneRoots(firstNonEmptyRoots(operation.DeclaredWrite, operation.Access.WriteRoots))
	secretScope := make([]sandbox.SecretScopeOutcome, 0, len(operation.LocalState))
	for _, item := range operation.LocalState {
		if !item.Sensitive {
			continue
		}
		secretScope = append(secretScope, sandbox.SecretScopeOutcome{
			ConsumerKind:     sandbox.ConsumerKindManagedProvider,
			ConsumerID:       consumerID,
			SecretRef:        strings.TrimSpace(item.StateClass),
			EnvironmentScope: sandbox.SecretEnvironmentScopeBoth,
			DefaultSource:    sandbox.SecretDefaultSourceInstanceOverride,
			DefaultRuleID:    "managed_provider:" + consumerID,
			DeliveryKind:     "local_state_access",
			RedactionRule:    item.RedactionRule,
			Resolution:       sandbox.SecretResolutionResolved,
		})
	}
	approvalMode := sandbox.ApprovalModeAllow
	requiredStrength := "declared_only"
	if evaluation != nil {
		approvalMode = evaluation.Declaration.ApprovalMode
		requiredStrength = firstNonEmpty(evaluation.Declaration.EnforcementStrength, requiredStrength)
	}
	policyRecord := &sandbox.ConsumerPolicyRecord{
		PolicyRecordID:      "policy_" + firstNonEmpty(strings.TrimSpace(operation.OperationID), newManagedProviderOperationID()),
		ConsumerKind:        sandbox.ConsumerKindManagedProvider,
		ConsumerID:          consumerID,
		OperationKind:       operationKind,
		DeclarationID:       declarationID,
		RequestedBy:         firstNonEmpty(strings.TrimSpace(operation.RequestedBy), managedProviderRequestedByPrefix+consumerID),
		Decision:            sandbox.DecisionResolutionAllow,
		ApprovalStatus:      sandbox.DecisionApprovalStatusNotApplicable,
		SecretResolution:    secretResolutionFromLocalState(secretScope),
		EnforcementStrength: requiredStrength,
		ProviderOperationID: strings.TrimSpace(operation.OperationID),
		StartedAt:           time.Now().UTC(),
		Status:              sandbox.PolicyRecordStatusPreflightAllowed,
	}
	if evaluation != nil {
		policyRecord.Decision = evaluation.Operation.Decision
		policyRecord.ApprovalStatus = evaluation.Operation.ApprovalStatus
		policyRecord.FailureClass = evaluation.Operation.FailureClass
	}
	return &sandbox.ConsumerContractView{
		Declaration: &sandbox.ConsumerRequirementDeclaration{
			DeclarationID:               declarationID,
			ConsumerKind:                sandbox.ConsumerKindManagedProvider,
			ConsumerID:                  consumerID,
			OperationKind:               operationKind,
			ProfileID:                   strings.TrimSpace(operation.ProfileID),
			ExecutionMode:               sandbox.ExecutionModeSubprocess,
			AllowedBackendKinds:         []sandbox.BackendKind{sandbox.BackendKindSubprocess},
			ReadRoots:                   readRoots,
			WriteRoots:                  writeRoots,
			NetworkMode:                 operation.Access.NetworkMode,
			AllowedHosts:                cloneStrings(operation.Access.AllowedHosts),
			AllowedPorts:                cloneInts(operation.Access.AllowedPorts),
			AllowLoopback:               operation.Access.AllowLoopback,
			SecretRefs:                  localStateClassList(operation.LocalState),
			ApprovalMode:                approvalMode,
			RequiredEnforcementStrength: requiredStrength,
			Active:                      true,
			Source:                      sandbox.SourceBuiltin,
		},
		SecretScope:  secretScope,
		PolicyRecord: policyRecord,
	}
}

func secretResolutionFromLocalState(items []sandbox.SecretScopeOutcome) sandbox.SecretResolution {
	if len(items) == 0 {
		return sandbox.SecretResolutionNotApplicable
	}
	return sandbox.SecretResolutionResolved
}

func consumerViewJSON(view *sandbox.ConsumerContractView) map[string]any {
	if view == nil {
		return nil
	}
	payload, err := json.Marshal(view)
	if err != nil {
		return nil
	}
	var item map[string]any
	if err := json.Unmarshal(payload, &item); err != nil {
		return nil
	}
	return item
}

func finalizeManagedProviderMetadata(metadata map[string]string, failureClass string) map[string]string {
	updated := cloneStringMap(metadata)
	if strings.TrimSpace(failureClass) == "" {
		delete(updated, managedProviderMetadataFailureClass)
		return updated
	}
	updated[managedProviderMetadataFailureClass] = strings.TrimSpace(failureClass)
	return updated
}

func finalizeManagedProviderExecutionSuccess(manager *sandbox.Manager, result RunResult) {
	if manager == nil || strings.TrimSpace(result.ExecutionID) == "" {
		return
	}
	_, _ = manager.FinalizeExecution(context.Background(), result.ExecutionID, sandbox.ExecutionFinalization{
		Status: sandbox.ExecutionStatusCompleted,
	})
}

func finalizeManagedProviderExecutionFailure(manager *sandbox.Manager, result RunResult, err error) {
	if manager == nil || strings.TrimSpace(result.ExecutionID) == "" || err == nil {
		return
	}
	finalization := sandbox.ExecutionFinalization{
		Status:     sandbox.ExecutionStatusFailed,
		ErrorClass: sandbox.ErrorClassProviderFailed,
		ErrorCode:  "provider_error",
		Error:      strings.TrimSpace(err.Error()),
	}
	var providerErr *llm.ProviderError
	if errors.As(err, &providerErr) {
		finalization.ErrorCode = firstNonEmpty(strings.TrimSpace(providerErr.Code), finalization.ErrorCode)
		finalization.Error = firstNonEmpty(strings.TrimSpace(providerErr.Message), finalization.Error)
		if strings.TrimSpace(providerErr.Code) == "upstream_auth_failed" {
			finalization.ErrorClass = sandbox.ErrorClassProviderAuth
		}
	}
	_, _ = manager.FinalizeExecution(context.Background(), result.ExecutionID, finalization)
}

func newManagedProviderOperationID() string {
	return "managed_provider_op_" + strings.ReplaceAll(time.Now().UTC().Format("20060102150405.000000000"), ".", "")
}

func cloneLocalStateSummaries(items []sandbox.SensitiveLocalStateAccessSummary) []sandbox.SensitiveLocalStateAccessSummary {
	cloned := make([]sandbox.SensitiveLocalStateAccessSummary, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, item)
	}
	return cloned
}

func cloneStrings(values []string) []string {
	return append([]string(nil), values...)
}

func cloneInts(values []int) []int {
	return append([]int(nil), values...)
}

func localStateSummary(providerID string, action sandbox.ManagedProviderActionKind, stateClass string, accessMode sandbox.LocalStateAccessMode, path string, sensitive bool) sandbox.SensitiveLocalStateAccessSummary {
	return sandbox.SensitiveLocalStateAccessSummary{
		ProviderID:    strings.TrimSpace(providerID),
		ActionKind:    action,
		StateClass:    strings.TrimSpace(stateClass),
		AccessMode:    accessMode,
		PathSummary:   redactedPathSummary(path),
		Declared:      true,
		Sensitive:     sensitive,
		RedactionRule: managedProviderRedactionRule,
	}
}

func redactedPathSummary(path string) string {
	base := strings.TrimSpace(filepath.Base(strings.TrimSpace(path)))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "redacted"
	}
	return base
}

func firstNonEmptyRoots(preferred []string, fallback []string) []string {
	if len(preferred) > 0 {
		return preferred
	}
	return fallback
}

func pathsWithinDeclared(paths []string, declared []string) bool {
	if len(paths) == 0 {
		return true
	}
	if len(declared) == 0 {
		return false
	}
	for _, path := range paths {
		if !pathWithinAny(path, declared) {
			return false
		}
	}
	return true
}

func pathWithinAny(path string, roots []string) bool {
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if cleanPath == "" {
		return false
	}
	for _, root := range roots {
		cleanRoot := filepath.Clean(strings.TrimSpace(root))
		if cleanRoot == "" {
			continue
		}
		if cleanPath == cleanRoot {
			return true
		}
		rel, err := filepath.Rel(cleanRoot, cleanPath)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func localStateClassList(items []sandbox.SensitiveLocalStateAccessSummary) []string {
	seen := map[string]struct{}{}
	values := make([]string, 0, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item.StateClass)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, key)
	}
	return values
}

func mergeStringMaps(base map[string]string, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	merged := cloneStringMap(base)
	for key, value := range extra {
		merged[key] = value
	}
	return merged
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func operationMetadataFromPlan(plan managedProviderOperationPlan) map[string]string {
	operation := sandbox.ManagedProviderOperation{
		OperationID:               firstNonEmpty(strings.TrimSpace(plan.OperationID), newManagedProviderOperationID()),
		ProviderID:                plan.ProviderID,
		ActionKind:                plan.Action,
		RequestedBy:               plan.RequestedBy,
		RequirementProfileID:      plan.ProfileID,
		Decision:                  sandbox.DecisionResolutionAllow,
		ApprovalStatus:            sandbox.DecisionApprovalStatusNotApplicable,
		EnforcementStrength:       "declared_only",
		SensitiveStateClasses:     cloneStrings(plan.SensitiveKinds),
		StartedAt:                 time.Now().UTC(),
		Status:                    sandbox.ManagedProviderOperationStatusRunning,
		LocalStateAccessSummaries: cloneLocalStateSummaries(plan.LocalState),
	}
	return operationMetadata(operation)
}
