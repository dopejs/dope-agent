package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dopejs/dope-agent/daemon/internal/config"
	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
)

const mcpSecretsFileName = "mcp-secrets.json"

func bundledCatalogEntries(cfg config.Config) []CatalogEntry {
	environment := string(cfg.Environment)
	filesystemSpec := CreateServerInput{
		DisplayName:      "Filesystem",
		Enabled:          true,
		OriginKind:       OriginKindCatalog,
		CatalogEntryID:   "filesystem",
		EnvironmentScope: environment,
		InstallMethod:    InstallMethodAPI,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:filesystem:lifecycle.start",
		Declaration: &Declaration{
			ExecutionMode:               sandbox.ExecutionModeSubprocess,
			AllowedBackendKinds:         []sandbox.BackendKind{sandbox.BackendKindSubprocess},
			ReadRoots:                   []string{cfg.DataDir},
			WriteRoots:                  []string{cfg.DataDir},
			NetworkMode:                 sandbox.NetworkModeDeny,
			ApprovalMode:                sandbox.ApprovalModeAllow,
			RequiredEnforcementStrength: "declared_only",
			Active:                      true,
		},
		TransportKind: TransportKindStdio,
		Command:       "npx",
		Args:          []string{"-y", "@modelcontextprotocol/server-filesystem", cfg.DataDir},
		WorkingDir:    cfg.DataDir,
		AutoRestart:   true,
	}
	githubSpec := CreateServerInput{
		DisplayName:      "GitHub",
		Enabled:          true,
		OriginKind:       OriginKindCatalog,
		CatalogEntryID:   "github",
		EnvironmentScope: environment,
		InstallMethod:    InstallMethodAPI,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:github:lifecycle.start",
		Declaration:      filesystemSpec.Declaration,
		TransportKind:    TransportKindStdio,
		Command:          "npx",
		Args:             []string{"-y", "@modelcontextprotocol/server-github"},
		WorkingDir:       cfg.DataDir,
		SecretRefs:       []string{"GITHUB_TOKEN"},
		AutoRestart:      true,
	}
	postgresSpec := CreateServerInput{
		DisplayName:      "Postgres",
		Enabled:          true,
		OriginKind:       OriginKindCatalog,
		CatalogEntryID:   "postgres",
		EnvironmentScope: environment,
		InstallMethod:    InstallMethodAPI,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:postgres:lifecycle.start",
		Declaration: &Declaration{
			ExecutionMode:               sandbox.ExecutionModeSubprocess,
			AllowedBackendKinds:         []sandbox.BackendKind{sandbox.BackendKindSubprocess},
			ReadRoots:                   []string{cfg.DataDir},
			WriteRoots:                  []string{cfg.DataDir},
			NetworkMode:                 sandbox.NetworkModeAllowList,
			AllowLoopback:               true,
			AllowedHosts:                []string{"localhost", "127.0.0.1"},
			ApprovalMode:                sandbox.ApprovalModeAllow,
			RequiredEnforcementStrength: "declared_only",
			Active:                      true,
		},
		TransportKind: TransportKindStdio,
		Command:       "npx",
		Args:          []string{"-y", "@modelcontextprotocol/server-postgres"},
		WorkingDir:    cfg.DataDir,
		SecretRefs:    []string{"POSTGRES_DSN"},
		AutoRestart:   true,
	}
	slackSpec := CreateServerInput{
		DisplayName:      "Slack",
		Enabled:          true,
		OriginKind:       OriginKindCatalog,
		CatalogEntryID:   "slack",
		EnvironmentScope: environment,
		InstallMethod:    InstallMethodAPI,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:slack:lifecycle.start",
		Declaration: &Declaration{
			ExecutionMode:               sandbox.ExecutionModeSubprocess,
			AllowedBackendKinds:         []sandbox.BackendKind{sandbox.BackendKindSubprocess},
			ReadRoots:                   []string{cfg.DataDir},
			WriteRoots:                  []string{cfg.DataDir},
			NetworkMode:                 sandbox.NetworkModeAllowList,
			AllowedHosts:                []string{"slack.com", "api.slack.com"},
			AllowedPorts:                []int{443},
			ApprovalMode:                sandbox.ApprovalModeAllow,
			RequiredEnforcementStrength: "declared_only",
			Active:                      true,
		},
		TransportKind: TransportKindStdio,
		Command:       "npx",
		Args:          []string{"-y", "@modelcontextprotocol/server-slack"},
		WorkingDir:    cfg.DataDir,
		SecretRefs:    []string{"SLACK_BOT_TOKEN"},
		AutoRestart:   true,
	}
	context7Spec := CreateServerInput{
		DisplayName:      "Context7",
		Enabled:          true,
		OriginKind:       OriginKindCatalog,
		CatalogEntryID:   "context7",
		EnvironmentScope: environment,
		InstallMethod:    InstallMethodAPI,
		SandboxProfileID: sandbox.ProfileIDSubprocessDefault,
		DeclarationID:    "mcp_server:context7:lifecycle.start",
		Declaration: &Declaration{
			ExecutionMode:               sandbox.ExecutionModeDeclarationOnly,
			AllowedBackendKinds:         []sandbox.BackendKind{sandbox.BackendKindSubprocess},
			NetworkMode:                 sandbox.NetworkModeAllowList,
			AllowedHosts:                []string{"mcp.context7.com"},
			AllowedPorts:                []int{443},
			ApprovalMode:                sandbox.ApprovalModeAllow,
			RequiredEnforcementStrength: "declared_only",
			Active:                      true,
		},
		TransportKind: TransportKindStreamableHTTP,
		Endpoint:      "https://mcp.context7.com/mcp",
		AutoRestart:   true,
	}

	items := []CatalogEntry{
		{
			ID:                     "filesystem",
			DisplayName:            "Filesystem",
			Description:            "Local project filesystem access for the active test workspace.",
			TransportKind:          TransportKindStdio,
			SourceKind:             "bundled",
			Tags:                   []string{"local", "filesystem", "starter"},
			ImmediateUse:           false,
			Prerequisites:          []CatalogPrerequisite{{Kind: "binary", Name: "npx", Required: true, Description: "Node.js with npx available on PATH"}},
			EnvironmentEligibility: []string{"test", "prod"},
			InstallSupport:         CatalogInstallSupport{ScriptSupported: true, ScriptArgs: []string{"filesystem"}},
			DefaultInstallSpec:     filesystemSpec,
		},
		{
			ID:                     "context7",
			DisplayName:            "Context7",
			Description:            "Remote docs and library context over streamable-http.",
			TransportKind:          TransportKindStreamableHTTP,
			SourceKind:             "bundled",
			Tags:                   []string{"remote", "docs", "starter"},
			ImmediateUse:           true,
			Prerequisites:          []CatalogPrerequisite{{Kind: "endpoint", Name: "streamable-http", Required: true, Description: "Reachable streamable-http MCP endpoint"}},
			EnvironmentEligibility: []string{"test", "prod"},
			InstallSupport:         CatalogInstallSupport{ScriptSupported: true, ScriptArgs: []string{"context7"}},
			DefaultInstallSpec:     context7Spec,
		},
		{
			ID:                     "github",
			DisplayName:            "GitHub",
			Description:            "GitHub repository and issue access through a credential-backed MCP server.",
			TransportKind:          TransportKindStdio,
			SourceKind:             "bundled",
			Tags:                   []string{"credentials", "git", "remote"},
			ImmediateUse:           false,
			Prerequisites:          []CatalogPrerequisite{{Kind: "binary", Name: "npx", Required: true, Description: "Node.js with npx available on PATH"}},
			SecretRequirements:     []CatalogSecretRequirement{{SecretRef: "GITHUB_TOKEN", Required: true, Description: "GitHub personal access token"}},
			EnvironmentEligibility: []string{"test", "prod"},
			InstallSupport:         CatalogInstallSupport{ScriptSupported: true, ScriptArgs: []string{"github"}},
			DefaultInstallSpec:     githubSpec,
		},
		{
			ID:                     "postgres",
			DisplayName:            "Postgres",
			Description:            "Database inspection and query access for a configured Postgres instance.",
			TransportKind:          TransportKindStdio,
			SourceKind:             "bundled",
			Tags:                   []string{"database", "credentials"},
			ImmediateUse:           false,
			Prerequisites:          []CatalogPrerequisite{{Kind: "binary", Name: "npx", Required: true, Description: "Node.js with npx available on PATH"}},
			SecretRequirements:     []CatalogSecretRequirement{{SecretRef: "POSTGRES_DSN", Required: true, Description: "Database connection string"}},
			EnvironmentEligibility: []string{"test", "prod"},
			InstallSupport:         CatalogInstallSupport{ScriptSupported: true, ScriptArgs: []string{"postgres"}},
			DefaultInstallSpec:     postgresSpec,
		},
		{
			ID:                     "slack",
			DisplayName:            "Slack",
			Description:            "Slack workspace access for channels, threads, and knowledge retrieval.",
			TransportKind:          TransportKindStdio,
			SourceKind:             "bundled",
			Tags:                   []string{"credentials", "chat", "remote"},
			ImmediateUse:           false,
			Prerequisites:          []CatalogPrerequisite{{Kind: "binary", Name: "npx", Required: true, Description: "Node.js with npx available on PATH"}},
			SecretRequirements:     []CatalogSecretRequirement{{SecretRef: "SLACK_BOT_TOKEN", Required: true, Description: "Slack bot token"}},
			EnvironmentEligibility: []string{"test", "prod"},
			InstallSupport:         CatalogInstallSupport{ScriptSupported: true, ScriptArgs: []string{"slack"}},
			DefaultInstallSpec:     slackSpec,
		},
	}
	for i := range items {
		status, reason := evaluateCatalogAvailability(cfg, items[i])
		items[i].AvailabilityStatus = status
		items[i].AvailabilityReason = reason
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items
}

func evaluateCatalogAvailability(cfg config.Config, entry CatalogEntry) (AvailabilityStatus, string) {
	for _, requirement := range entry.Prerequisites {
		if !requirement.Required {
			continue
		}
		switch requirement.Kind {
		case "binary":
			if _, err := exec.LookPath(strings.TrimSpace(requirement.Name)); err != nil {
				return AvailabilityStatusUnavailable, firstNonEmpty(requirement.Description, fmt.Sprintf("%s is unavailable", requirement.Name))
			}
		case "endpoint":
			if entry.TransportKind == TransportKindStreamableHTTP && strings.TrimSpace(entry.DefaultInstallSpec.Endpoint) == "" {
				return AvailabilityStatusUnsupported, "streamable-http endpoint is not configured"
			}
		}
	}
	return evaluateCatalogInstallSpecAvailability(cfg, entry.DefaultInstallSpec, entry.SecretRequirements)
}

func secretRefsFromRequirements(items []CatalogSecretRequirement) []string {
	refs := make([]string, 0, len(items))
	for _, item := range items {
		refs = append(refs, item.SecretRef)
	}
	return refs
}

func ResolveMCPSecrets(secretRoot string, secretRefs []string) (map[string]string, error) {
	refs := cleanStrings(secretRefs)
	if len(refs) == 0 {
		return map[string]string{}, nil
	}
	payload, err := os.ReadFile(filepath.Join(strings.TrimSpace(secretRoot), mcpSecretsFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("read mcp secrets: %w", err)
	}
	var values map[string]string
	if err := json.Unmarshal(payload, &values); err != nil {
		return nil, fmt.Errorf("decode mcp secrets: %w", err)
	}
	resolved := make(map[string]string, len(refs))
	for _, secretRef := range refs {
		if value := strings.TrimSpace(values[secretRef]); value != "" {
			resolved[secretRef] = value
		}
	}
	return resolved, nil
}

func evaluateCatalogInstallSpecAvailability(cfg config.Config, spec CreateServerInput, requirements []CatalogSecretRequirement) (AvailabilityStatus, string) {
	switch spec.TransportKind {
	case TransportKindStdio:
		if strings.TrimSpace(spec.Command) == "" {
			return AvailabilityStatusUnavailable, "stdio command is not configured"
		}
	case TransportKindStreamableHTTP:
		if strings.TrimSpace(spec.Endpoint) == "" {
			return AvailabilityStatusUnsupported, "streamable-http endpoint is not configured"
		}
	default:
		return AvailabilityStatusUnsupported, "transport kind is unsupported"
	}
	if requiresOfflineVerifiedLocalCommand(spec) {
		return AvailabilityStatusUnavailable, "default bundled stdio command requires a local command override because sandbox network is denied"
	}
	if len(requirements) == 0 {
		return AvailabilityStatusReady, ""
	}
	resolved, err := ResolveMCPSecrets(cfg.DataDir, secretRefsFromRequirements(requirements))
	if err != nil {
		return AvailabilityStatusBlocked, err.Error()
	}
	for _, requirement := range requirements {
		if !requirement.Required {
			continue
		}
		if _, ok := resolved[requirement.SecretRef]; !ok {
			return AvailabilityStatusBlocked, firstNonEmpty(requirement.Description, fmt.Sprintf("%s is required", requirement.SecretRef))
		}
	}
	return AvailabilityStatusReady, ""
}

func requiresOfflineVerifiedLocalCommand(spec CreateServerInput) bool {
	if spec.TransportKind != TransportKindStdio {
		return false
	}
	if spec.Declaration == nil || spec.Declaration.NetworkMode != sandbox.NetworkModeDeny {
		return false
	}
	command := strings.TrimSpace(spec.Command)
	if command != "npx" && command != "npm" {
		return false
	}
	for _, arg := range cleanStrings(spec.Args) {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return strings.Contains(arg, "@modelcontextprotocol/")
	}
	return false
}
