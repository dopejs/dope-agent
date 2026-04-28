package skills

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dopejs/dope-agent/daemon/internal/sandbox"
	"github.com/dopejs/dope-agent/daemon/internal/secrets"
	"github.com/dopejs/dope-agent/daemon/internal/tenantctx"
)

var (
	ErrSkillNotFound         = errors.New("skill not found")
	ErrSkillsRegistryMissing = errors.New("skills registry is not configured")
)

type Source string

const (
	SourceHome    Source = "home"
	SourceDataDir Source = "data_dir"
)

type File struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
}

type Skill struct {
	SkillID            string                  `json:"skillId"`
	Name               string                  `json:"name"`
	Description        string                  `json:"description"`
	Source             Source                  `json:"source"`
	RootPath           string                  `json:"rootPath"`
	SkillPath          string                  `json:"skillPath"`
	InstructionPath    string                  `json:"instructionPath"`
	Frontmatter        map[string]string       `json:"frontmatter"`
	FrontmatterRaw     string                  `json:"frontmatterRaw,omitempty"`
	Body               string                  `json:"body,omitempty"`
	Files              []File                  `json:"files"`
	ExecutionManifest  *ExecutableManifest     `json:"executionManifest,omitempty"`
	AvailabilityStatus SkillAvailabilityStatus `json:"availabilityStatus,omitempty"`
	AvailabilityReason string                  `json:"availabilityReason,omitempty"`
	Sandbox            map[string]any          `json:"sandbox,omitempty"`
}

type SkillAvailabilityStatus string

const (
	SkillAvailabilityStatusNotExecutable SkillAvailabilityStatus = "not_executable"
	SkillAvailabilityStatusAvailable     SkillAvailabilityStatus = "available"
	SkillAvailabilityStatusUnavailable   SkillAvailabilityStatus = "unavailable"
)

type ExecutableManifest struct {
	Entrypoint                  string               `json:"entrypoint"`
	Args                        []string             `json:"args,omitempty"`
	WorkingDir                  string               `json:"workingDir,omitempty"`
	ProfileID                   string               `json:"profileId"`
	BackendKind                 sandbox.BackendKind  `json:"backendKind,omitempty"`
	ReadRoots                   []string             `json:"readRoots,omitempty"`
	WriteRoots                  []string             `json:"writeRoots,omitempty"`
	NetworkMode                 sandbox.NetworkMode  `json:"networkMode,omitempty"`
	AllowedHosts                []string             `json:"allowedHosts,omitempty"`
	AllowedPorts                []int                `json:"allowedPorts,omitempty"`
	SecretRefs                  []string             `json:"secretRefs,omitempty"`
	ApprovalMode                sandbox.ApprovalMode `json:"approvalMode"`
	TimeoutMs                   int                  `json:"timeoutMs,omitempty"`
	RequiredEnforcementStrength string               `json:"requiredEnforcementStrength,omitempty"`
}

type Overlay struct {
	OverlayID  string    `json:"overlayId"`
	Source     Source    `json:"source"`
	Path       string    `json:"path"`
	SizeBytes  int64     `json:"sizeBytes"`
	ModifiedAt time.Time `json:"modifiedAt"`
	Body       string    `json:"body,omitempty"`
}

type Snapshot struct {
	LoadedAt time.Time `json:"loadedAt"`
	Skills   []Skill   `json:"skills"`
	Overlays []Overlay `json:"overlays"`
}

type Registry struct {
	homeRoot string
	dataRoot string

	mu       sync.RWMutex
	snapshot Snapshot
	index    map[string]Skill
}

const executableSkillSecretsFileName = "skill-secrets.json"

func NewRegistry(dataRoot string) (*Registry, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user home: %w", err)
	}
	return NewRegistryWithRoots(filepath.Join(homeDir, ".agents"), dataRoot)
}

func NewRegistryWithRoots(homeRoot string, dataRoot string) (*Registry, error) {
	registry := &Registry{
		homeRoot: strings.TrimSpace(homeRoot),
		dataRoot: strings.TrimSpace(dataRoot),
		index:    map[string]Skill{},
	}
	if err := registry.Reload(); err != nil {
		return nil, err
	}
	return registry, nil
}

func (r *Registry) Reload() error {
	if r == nil {
		return ErrSkillsRegistryMissing
	}

	homeSkills, err := scanSkills(filepath.Join(r.homeRoot, "skills"), SourceHome, r.dataRoot)
	if err != nil {
		return err
	}
	dataSkills, err := scanSkills(filepath.Join(r.dataRoot, "skills"), SourceDataDir, r.dataRoot)
	if err != nil {
		return err
	}
	homeOverlay, err := loadOverlay(filepath.Join(r.homeRoot, "AGENTS.md"), SourceHome)
	if err != nil {
		return err
	}
	dataOverlay, err := loadOverlay(filepath.Join(r.dataRoot, "AGENTS.md"), SourceDataDir)
	if err != nil {
		return err
	}

	index := make(map[string]Skill, len(homeSkills)+len(dataSkills))
	for _, skill := range homeSkills {
		index[normalizeSkillID(skill.SkillID)] = skill
	}
	for _, skill := range dataSkills {
		index[normalizeSkillID(skill.SkillID)] = skill
	}

	skillsList := make([]Skill, 0, len(index))
	for _, skill := range index {
		skillsList = append(skillsList, cloneSkill(skill))
	}
	sort.Slice(skillsList, func(i, j int) bool {
		return skillsList[i].SkillID < skillsList[j].SkillID
	})

	overlays := make([]Overlay, 0, 2)
	if homeOverlay != nil {
		overlays = append(overlays, *homeOverlay)
	}
	if dataOverlay != nil {
		overlays = append(overlays, *dataOverlay)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.index = index
	r.snapshot = Snapshot{
		LoadedAt: time.Now().UTC(),
		Skills:   skillsList,
		Overlays: overlays,
	}
	return nil
}

func (r *Registry) Snapshot() Snapshot {
	if r == nil {
		return Snapshot{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return cloneSnapshot(r.snapshot)
}

func (r *Registry) List() []Skill {
	return r.Snapshot().Skills
}

func (r *Registry) Overlays() []Overlay {
	return r.Snapshot().Overlays
}

func (r *Registry) Get(skillID string) (Skill, bool) {
	if r == nil {
		return Skill{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	skill, ok := r.index[normalizeSkillID(skillID)]
	if !ok {
		return Skill{}, false
	}
	return cloneSkill(skill), true
}

func (r *Registry) ResolveSelected(skillIDs []string) ([]Skill, error) {
	if r == nil {
		if len(skillIDs) == 0 {
			return nil, nil
		}
		return nil, ErrSkillsRegistryMissing
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	selected := make([]Skill, 0, len(skillIDs))
	seen := map[string]struct{}{}
	for _, item := range skillIDs {
		normalized := normalizeSkillID(item)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		skill, ok := r.index[normalized]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrSkillNotFound, strings.TrimSpace(item))
		}
		selected = append(selected, cloneSkill(skill))
		seen[normalized] = struct{}{}
	}
	return selected, nil
}

func scanSkills(root string, source Source, secretRoot string) ([]Skill, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read skills root %s: %w", root, err)
	}

	skillsList := make([]Skill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skill, err := loadSkill(filepath.Join(root, entry.Name()), source, secretRoot)
		if err != nil {
			return nil, err
		}
		if skill.SkillID == "" {
			continue
		}
		skillsList = append(skillsList, skill)
	}
	return skillsList, nil
}

func loadSkill(skillRoot string, source Source, secretRoot string) (Skill, error) {
	instructionPath := filepath.Join(skillRoot, "SKILL.md")
	content, _, err := readFileWithStat(instructionPath)
	if errors.Is(err, fs.ErrNotExist) {
		return Skill{}, nil
	}
	if err != nil {
		return Skill{}, fmt.Errorf("read skill %s: %w", instructionPath, err)
	}

	frontmatterRaw, frontmatter, body := parseSkillFrontmatter(content)
	skillName := strings.TrimSpace(frontmatter["name"])
	if skillName == "" {
		skillName = filepath.Base(skillRoot)
	}
	skillID := normalizeSkillID(skillName)
	if skillID == "" {
		return Skill{}, nil
	}

	files, err := bundledFiles(skillRoot)
	if err != nil {
		return Skill{}, err
	}
	executionManifest, availabilityStatus, availabilityReason := parseExecutableManifest(skillRoot, secretRoot, frontmatter)

	return Skill{
		SkillID:            skillID,
		Name:               strings.TrimSpace(skillName),
		Description:        strings.TrimSpace(frontmatter["description"]),
		Source:             source,
		RootPath:           filepath.Dir(skillRoot),
		SkillPath:          skillRoot,
		InstructionPath:    instructionPath,
		Frontmatter:        frontmatter,
		FrontmatterRaw:     frontmatterRaw,
		Body:               strings.TrimSpace(body),
		Files:              files,
		ExecutionManifest:  executionManifest,
		AvailabilityStatus: availabilityStatus,
		AvailabilityReason: availabilityReason,
		Sandbox:            buildSkillSandboxView(skillID, skillRoot),
	}, nil
}

func parseExecutableManifest(skillRoot, secretRoot string, frontmatter map[string]string) (*ExecutableManifest, SkillAvailabilityStatus, string) {
	const prefix = "execution."
	hasExecutionKeys := false
	for key := range frontmatter {
		if strings.HasPrefix(strings.TrimSpace(key), prefix) {
			hasExecutionKeys = true
			break
		}
	}
	if !hasExecutionKeys {
		return nil, SkillAvailabilityStatusNotExecutable, ""
	}

	manifest := &ExecutableManifest{
		Entrypoint:                  strings.TrimSpace(frontmatter["execution.entrypoint"]),
		Args:                        splitCSV(frontmatter["execution.args"]),
		ProfileID:                   strings.TrimSpace(frontmatter["execution.profile_id"]),
		ReadRoots:                   resolveSkillPaths(skillRoot, splitCSV(frontmatter["execution.read_roots"])),
		WriteRoots:                  resolveSkillPaths(skillRoot, splitCSV(frontmatter["execution.write_roots"])),
		NetworkMode:                 sandbox.NetworkMode(firstNonEmpty(frontmatter["execution.network_mode"], string(sandbox.NetworkModeDeny))),
		AllowedHosts:                splitCSV(frontmatter["execution.allowed_hosts"]),
		AllowedPorts:                splitCSVInts(frontmatter["execution.allowed_ports"]),
		SecretRefs:                  splitCSV(frontmatter["execution.secret_refs"]),
		ApprovalMode:                sandbox.ApprovalMode(firstNonEmpty(frontmatter["execution.approval_mode"], string(sandbox.ApprovalModeAsk))),
		RequiredEnforcementStrength: firstNonEmpty(strings.TrimSpace(frontmatter["execution.required_enforcement_strength"]), "declared_only"),
	}
	if workingDir := strings.TrimSpace(frontmatter["execution.working_dir"]); workingDir != "" {
		manifest.WorkingDir = resolveSkillPath(skillRoot, workingDir)
	}
	if timeoutValue := strings.TrimSpace(frontmatter["execution.timeout_ms"]); timeoutValue != "" {
		timeoutMs, err := strconv.Atoi(timeoutValue)
		if err != nil || timeoutMs <= 0 {
			return manifest, SkillAvailabilityStatusUnavailable, "execution.timeout_ms must be a positive integer"
		}
		manifest.TimeoutMs = timeoutMs
	}

	switch manifest.NetworkMode {
	case sandbox.NetworkModeDeny, sandbox.NetworkModeAllowList, sandbox.NetworkModeFull:
	default:
		return manifest, SkillAvailabilityStatusUnavailable, "execution.network_mode is invalid"
	}
	switch manifest.ApprovalMode {
	case sandbox.ApprovalModeAllow, sandbox.ApprovalModeAsk, sandbox.ApprovalModeDeny:
	default:
		return manifest, SkillAvailabilityStatusUnavailable, "execution.approval_mode is invalid"
	}
	if manifest.Entrypoint == "" {
		return manifest, SkillAvailabilityStatusUnavailable, "execution.entrypoint is required"
	}
	if manifest.ProfileID == "" {
		return manifest, SkillAvailabilityStatusUnavailable, "execution.profile_id is required"
	}
	manifest.BackendKind = backendKindForProfile(manifest.ProfileID)
	if !supportsRequiredStrength(manifest.RequiredEnforcementStrength) {
		return manifest, SkillAvailabilityStatusUnavailable, "execution.required_enforcement_strength exceeds current backend support"
	}
	if entrypoint, ok := resolveManifestEntrypoint(skillRoot, manifest.Entrypoint); ok {
		manifest.Entrypoint = entrypoint
	} else {
		return manifest, SkillAvailabilityStatusUnavailable, "execution.entrypoint does not resolve to an executable target"
	}
	if manifest.WorkingDir != "" {
		if info, err := os.Stat(manifest.WorkingDir); err != nil || !info.IsDir() {
			return manifest, SkillAvailabilityStatusUnavailable, "execution.working_dir does not exist"
		}
	}
	resolvedSecrets, err := ResolveExecutableSkillSecrets(secretRoot, manifest.SecretRefs)
	if err != nil {
		return manifest, SkillAvailabilityStatusUnavailable, fmt.Sprintf("executable skill secrets are unavailable: %v", err)
	}
	for _, secretRef := range manifest.SecretRefs {
		if strings.TrimSpace(secretRef) == "" {
			return manifest, SkillAvailabilityStatusUnavailable, "execution.secret_refs contains an empty entry"
		}
		if _, ok := resolvedSecrets[secretRef]; !ok {
			return manifest, SkillAvailabilityStatusUnavailable, fmt.Sprintf("secret ref %s is unavailable in %s", secretRef, effectiveEnvironment())
		}
	}
	return manifest, SkillAvailabilityStatusAvailable, ""
}

func ResolveExecutableSkillSecrets(secretRoot string, secretRefs []string) (map[string]string, error) {
	items := make(map[string]string, len(secretRefs))
	if len(secretRefs) == 0 {
		return items, nil
	}
	values, err := loadExecutableSkillSecretFile(secretRoot)
	if err != nil {
		return nil, err
	}
	for _, secretRef := range secretRefs {
		trimmed := strings.TrimSpace(secretRef)
		if trimmed == "" {
			continue
		}
		if value, ok := values[trimmed]; ok && strings.TrimSpace(value) != "" {
			items[trimmed] = value
		}
	}
	return items, nil
}

func ResolveExecutableSkillSecretsForTenant(ctx context.Context, secretManager *secrets.Manager, secretRefs []string) (map[string]string, error) {
	refs := cleanStrings(secretRefs)
	items := make(map[string]string, len(refs))
	if len(refs) == 0 {
		return items, nil
	}
	if secretManager == nil {
		return nil, errors.New("tenant secret manager is required")
	}
	tenantContext, ok := tenantctx.FromContext(ctx)
	if !ok || strings.TrimSpace(tenantContext.TenantID) == "" {
		return nil, tenantctx.ErrTenantContextRequired
	}
	for _, secretRef := range refs {
		secret, err := secretManager.Resolve(ctx, secrets.ResolveInput{
			TenantID:  strings.TrimSpace(tenantContext.TenantID),
			SecretRef: secretRef,
		})
		if err != nil {
			return nil, fmt.Errorf("resolve executable skill secret %s: %w", secretRef, err)
		}
		if strings.TrimSpace(secret.Value) == "" {
			return nil, fmt.Errorf("resolve executable skill secret %s: empty value", secretRef)
		}
		items[secretRef] = secret.Value
	}
	return items, nil
}

func loadExecutableSkillSecretFile(secretRoot string) (map[string]string, error) {
	path := executableSkillSecretsPath(secretRoot)
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	values := map[string]string{}
	if err := json.Unmarshal(content, &values); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	for key, value := range values {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			delete(values, key)
			continue
		}
		if trimmedKey != key {
			delete(values, key)
			values[trimmedKey] = value
		}
	}
	return values, nil
}

func executableSkillSecretsPath(secretRoot string) string {
	return filepath.Join(strings.TrimSpace(secretRoot), executableSkillSecretsFileName)
}

func resolveManifestEntrypoint(skillRoot, value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	if filepath.IsAbs(trimmed) {
		_, err := os.Stat(trimmed)
		return trimmed, err == nil
	}
	if strings.Contains(trimmed, "/") || strings.HasPrefix(trimmed, ".") {
		resolved := resolveSkillPath(skillRoot, trimmed)
		_, err := os.Stat(resolved)
		return resolved, err == nil
	}
	return trimmed, true
}

func supportsRequiredStrength(value string) bool {
	switch strings.TrimSpace(value) {
	case "", "declared_only", "subprocess", "containerized", "docker":
		return true
	default:
		return false
	}
}

func backendKindForProfile(profileID string) sandbox.BackendKind {
	switch strings.TrimSpace(profileID) {
	case sandbox.ProfileIDDockerDefault:
		return sandbox.BackendKindDocker
	default:
		return sandbox.BackendKindSubprocess
	}
}

func resolveSkillPath(skillRoot, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed)
	}
	return filepath.Clean(filepath.Join(skillRoot, trimmed))
}

func resolveSkillPaths(skillRoot string, values []string) []string {
	resolved := make([]string, 0, len(values))
	for _, value := range values {
		if item := resolveSkillPath(skillRoot, value); item != "" {
			resolved = append(resolved, item)
		}
	}
	return resolved
}

func splitCSV(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			items = append(items, item)
		}
	}
	return items
}

func cleanStrings(values []string) []string {
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

func splitCSVInts(value string) []int {
	items := splitCSV(value)
	ports := make([]int, 0, len(items))
	for _, item := range items {
		parsed, err := strconv.Atoi(item)
		if err != nil {
			return nil
		}
		ports = append(ports, parsed)
	}
	return ports
}

func effectiveEnvironment() string {
	switch strings.TrimSpace(os.Getenv("DOPE_ENV")) {
	case "prod":
		return "prod"
	default:
		return "test"
	}
}

func loadOverlay(path string, source Source) (*Overlay, error) {
	content, stat, err := readFileWithStat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read overlay %s: %w", path, err)
	}
	return &Overlay{
		OverlayID:  string(source) + "_agents",
		Source:     source,
		Path:       path,
		SizeBytes:  stat.Size(),
		ModifiedAt: stat.ModTime().UTC(),
		Body:       strings.TrimSpace(content),
	}, nil
}

func bundledFiles(skillRoot string) ([]File, error) {
	files := []File{}
	err := filepath.WalkDir(skillRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) == "SKILL.md" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(skillRoot, path)
		if err != nil {
			return err
		}
		files = append(files, File{
			Path:      filepath.ToSlash(relativePath),
			SizeBytes: info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk skill files %s: %w", skillRoot, err)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func buildSkillSandboxView(skillID, skillRoot string) map[string]any {
	view := &sandbox.ConsumerContractView{
		Declaration: &sandbox.ConsumerRequirementDeclaration{
			DeclarationID:               "skill:" + normalizeSkillID(skillID) + ":selection",
			ConsumerKind:                sandbox.ConsumerKindSkill,
			ConsumerID:                  normalizeSkillID(skillID),
			OperationKind:               "skill_selection",
			ProfileID:                   sandbox.ProfileIDSubprocessDefault,
			ExecutionMode:               sandbox.ExecutionModeDeclarationOnly,
			AllowedBackendKinds:         []sandbox.BackendKind{sandbox.BackendKindSubprocess},
			ReadRoots:                   []string{skillRoot},
			WriteRoots:                  []string{},
			NetworkMode:                 sandbox.NetworkModeDeny,
			SecretRefs:                  []string{},
			ApprovalMode:                sandbox.ApprovalModeAllow,
			RequiredEnforcementStrength: "declared_only",
			Active:                      true,
			Source:                      sandbox.SourceBuiltin,
		},
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

func readFileWithStat(path string) (string, fs.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	return string(content), info, nil
}

func parseSkillFrontmatter(raw string) (string, map[string]string, string) {
	trimmed := strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(trimmed, "---\n") {
		return "", map[string]string{}, trimmed
	}
	rest := strings.TrimPrefix(trimmed, "---\n")
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return "", map[string]string{}, trimmed
	}
	header := rest[:end]
	body := strings.TrimPrefix(rest[end:], "\n---\n")
	fields := map[string]string{}
	for _, line := range strings.Split(header, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		fields[strings.TrimSpace(key)] = unquoteYAMLScalar(strings.TrimSpace(value))
	}
	return header, fields, body
}

func unquoteYAMLScalar(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= 2 {
		if (strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"")) ||
			(strings.HasPrefix(trimmed, "'") && strings.HasSuffix(trimmed, "'")) {
			return strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		}
	}
	return trimmed
}

func normalizeSkillID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	cloned := Snapshot{
		LoadedAt: snapshot.LoadedAt,
		Skills:   make([]Skill, 0, len(snapshot.Skills)),
		Overlays: make([]Overlay, 0, len(snapshot.Overlays)),
	}
	for _, skill := range snapshot.Skills {
		cloned.Skills = append(cloned.Skills, cloneSkill(skill))
	}
	for _, overlay := range snapshot.Overlays {
		cloned.Overlays = append(cloned.Overlays, overlay)
	}
	return cloned
}

func cloneSkill(skill Skill) Skill {
	cloned := skill
	cloned.Frontmatter = map[string]string{}
	for key, value := range skill.Frontmatter {
		cloned.Frontmatter[key] = value
	}
	cloned.Files = append([]File(nil), skill.Files...)
	if skill.ExecutionManifest != nil {
		manifest := *skill.ExecutionManifest
		manifest.Args = append([]string(nil), skill.ExecutionManifest.Args...)
		manifest.ReadRoots = append([]string(nil), skill.ExecutionManifest.ReadRoots...)
		manifest.WriteRoots = append([]string(nil), skill.ExecutionManifest.WriteRoots...)
		manifest.AllowedHosts = append([]string(nil), skill.ExecutionManifest.AllowedHosts...)
		manifest.AllowedPorts = append([]int(nil), skill.ExecutionManifest.AllowedPorts...)
		manifest.SecretRefs = append([]string(nil), skill.ExecutionManifest.SecretRefs...)
		cloned.ExecutionManifest = &manifest
	}
	if skill.Sandbox != nil {
		payload, err := json.Marshal(skill.Sandbox)
		if err == nil {
			var view map[string]any
			if json.Unmarshal(payload, &view) == nil {
				cloned.Sandbox = view
			} else {
				cloned.Sandbox = skill.Sandbox
			}
		} else {
			cloned.Sandbox = skill.Sandbox
		}
	}
	return cloned
}
