package billing

import (
	"fmt"
	"strings"
)

func RunOperationKey(tenantID, clientKey, runID string) string {
	return joinOperationKey("tenant", tenantID, "run", firstNonEmpty(clientKey, runID))
}

func WorkflowOperationKey(tenantID, runID, workflowID, clientKey string) string {
	return joinOperationKey("tenant", tenantID, "workflow", runID, firstNonEmpty(workflowID, clientKey))
}

func ToolCallOperationKey(tenantID, runID, stepID, toolCallID, clientKey string) string {
	return joinOperationKey("tenant", tenantID, "tool_call", runID, stepID, firstNonEmpty(toolCallID, clientKey))
}

func LiveValidationOperationKey(tenantID, validationID, clientKey string) string {
	return joinOperationKey("tenant", tenantID, "live_validation", firstNonEmpty(validationID, clientKey))
}

func IntegrationOperationKey(tenantID, domain, operationID, clientKey string) string {
	return joinOperationKey("tenant", tenantID, "integration", domain, firstNonEmpty(operationID, clientKey))
}

func ArtifactOperationKey(tenantID, artifactID, storageKey, clientKey string) string {
	return joinOperationKey("tenant", tenantID, "artifact", firstNonEmpty(artifactID, storageKey, clientKey))
}

func EvaluationOperationKey(tenantID, candidateID, attemptID, clientKey string) string {
	return joinOperationKey("tenant", tenantID, "evaluation", candidateID, firstNonEmpty(attemptID, clientKey))
}

func joinOperationKey(parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			value = "unknown"
		}
		clean = append(clean, strings.ReplaceAll(value, ":", "_"))
	}
	return strings.Join(clean, ":")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return fmt.Sprintf("missing_%d", len(values))
}
