package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRegistryLoadsDataDirAndHomeSkillsWithDataDirPrecedence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	homeRoot := filepath.Join(os.Getenv("HOME"), ".agents")
	dataRoot := t.TempDir()

	writeSkillFixture(t, filepath.Join(homeRoot, "skills", "shared-skill"), `---
name: shared-skill
description: "home description"
---

home body`)
	writeSkillFixture(t, filepath.Join(homeRoot, "skills", "home-only"), `---
name: home-only
description: "home only"
---

home only body`)
	writeFileFixture(t, filepath.Join(homeRoot, "AGENTS.md"), "home overlay")

	writeSkillFixture(t, filepath.Join(dataRoot, "skills", "shared-skill"), `---
name: shared-skill
description: "data description"
---

data body`)
	writeSkillFixture(t, filepath.Join(dataRoot, "skills", "data-only"), `---
name: data-only
description: "data only"
---

data only body`)
	writeFileFixture(t, filepath.Join(dataRoot, "AGENTS.md"), "data overlay")

	registry, err := NewRegistry(dataRoot)
	if err != nil {
		t.Fatalf("NewRegistry returned error: %v", err)
	}

	snapshot := registry.Snapshot()
	if len(snapshot.Skills) != 3 {
		t.Fatalf("expected 3 effective skills, got %d", len(snapshot.Skills))
	}
	if len(snapshot.Overlays) != 2 {
		t.Fatalf("expected 2 overlays, got %d", len(snapshot.Overlays))
	}

	shared, ok := registry.Get("shared-skill")
	if !ok {
		t.Fatal("expected shared-skill to exist")
	}
	if shared.Source != SourceDataDir {
		t.Fatalf("expected data-dir precedence, got %s", shared.Source)
	}
	if !strings.Contains(shared.Body, "data body") {
		t.Fatalf("expected data-dir body, got %q", shared.Body)
	}

	selected, err := registry.ResolveSelected([]string{"home-only", "shared-skill", "HOME-ONLY"})
	if err != nil {
		t.Fatalf("ResolveSelected returned error: %v", err)
	}
	if len(selected) != 2 {
		t.Fatalf("expected 2 deduplicated selected skills, got %d", len(selected))
	}
	if selected[0].SkillID != "home-only" || selected[1].SkillID != "shared-skill" {
		t.Fatalf("unexpected selection order: %#v", selected)
	}
}

func TestRegistryRejectsUnknownSkill(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	registry, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewRegistry returned error: %v", err)
	}
	if _, err := registry.ResolveSelected([]string{"missing"}); err == nil {
		t.Fatal("expected missing skill resolution to fail")
	}
}

func TestRegistryLoadsBundledFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataRoot := t.TempDir()
	writeSkillFixture(t, filepath.Join(dataRoot, "skills", "bundle-skill"), `---
name: bundle-skill
description: "has files"
---

bundle body`)
	writeFileFixture(t, filepath.Join(dataRoot, "skills", "bundle-skill", "references", "guide.md"), "guide")
	writeFileFixture(t, filepath.Join(dataRoot, "skills", "bundle-skill", "scripts", "run.sh"), "#!/bin/sh")

	registry, err := NewRegistry(dataRoot)
	if err != nil {
		t.Fatalf("NewRegistry returned error: %v", err)
	}

	skill, ok := registry.Get("bundle-skill")
	if !ok {
		t.Fatal("expected bundle-skill")
	}
	if len(skill.Files) != 2 {
		t.Fatalf("expected bundled file inventory, got %d", len(skill.Files))
	}
	if skill.Files[0].Path != "references/guide.md" || skill.Files[1].Path != "scripts/run.sh" {
		t.Fatalf("unexpected file inventory: %#v", skill.Files)
	}
}

func TestRegistryProjectsSandboxDeclarationForSkillSelection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataRoot := t.TempDir()
	writeSkillFixture(t, filepath.Join(dataRoot, "skills", "shared-skill"), `---
name: shared-skill
description: "shared"
---
shared body`)

	registry, err := NewRegistry(dataRoot)
	if err != nil {
		t.Fatalf("NewRegistry returned error: %v", err)
	}

	skill, ok := registry.Get("shared-skill")
	if !ok {
		t.Fatal("expected shared-skill")
	}
	if skill.Sandbox == nil {
		t.Fatalf("expected sandbox declaration for skill, got %+v", skill)
	}
	declaration, ok := skill.Sandbox["declaration"].(map[string]any)
	if !ok {
		t.Fatalf("expected declaration payload, got %+v", skill.Sandbox)
	}
	if declaration["consumerKind"] != "skill" || declaration["consumerId"] != "shared-skill" || declaration["operationKind"] != "skill_selection" {
		t.Fatalf("expected shared declaration vocabulary, got %+v", declaration)
	}
	readRoots, ok := declaration["readRoots"].([]any)
	if !ok || len(readRoots) != 1 {
		t.Fatalf("expected declared skill root read access, got %+v", declaration)
	}
	if !strings.HasSuffix(readRoots[0].(string), filepath.Join("skills", "shared-skill")) {
		t.Fatalf("expected read root to point at skill root, got %+v", readRoots)
	}
}

func TestRegistryParsesExecutableManifestAndDefaultsApprovalToAsk(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOPE_ENV", "test")
	dataRoot := t.TempDir()
	writeExecutableSkillSecretsFixture(t, dataRoot, map[string]string{"EXEC_SKILL_TOKEN": "available"})
	writeValidExecutableSkillFixture(t, dataRoot, "exec-skill", executableSkillFixtureOptions{
		SecretRefs: []string{"EXEC_SKILL_TOKEN"},
		Args:       []string{"alpha", "beta"},
		TimeoutMs:  500,
	})

	started := time.Now()
	registry, err := NewRegistry(dataRoot)
	if err != nil {
		t.Fatalf("NewRegistry returned error: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("expected executable manifest preflight <=100ms, got %s", elapsed)
	}

	skill, ok := registry.Get("exec-skill")
	if !ok {
		t.Fatal("expected exec-skill")
	}
	if skill.ExecutionManifest == nil {
		t.Fatalf("expected executable manifest, got %+v", skill)
	}
	if skill.ExecutionManifest.ApprovalMode != "ask" {
		t.Fatalf("expected default ask approval mode, got %+v", skill.ExecutionManifest)
	}
	if skill.AvailabilityStatus != SkillAvailabilityStatusAvailable {
		t.Fatalf("expected available status, got %+v", skill)
	}
	if !strings.HasSuffix(skill.ExecutionManifest.Entrypoint, filepath.Join("skills", "exec-skill", "scripts", "run.sh")) {
		t.Fatalf("expected resolved entrypoint path, got %+v", skill.ExecutionManifest)
	}
}

func TestRegistryMarksExecutableSkillUnavailableWhenSecretRefMissingForEnvironment(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOPE_ENV", "test")
	dataRoot := t.TempDir()
	writeValidExecutableSkillFixture(t, dataRoot, "secret-skill", executableSkillFixtureOptions{
		Description: "needs secret",
		SecretRefs:  []string{"MISSING_SKILL_SECRET"},
		ScriptBody:  "#!/bin/sh\nprintf nope",
	})

	registry, err := NewRegistry(dataRoot)
	if err != nil {
		t.Fatalf("NewRegistry returned error: %v", err)
	}

	skill, ok := registry.Get("secret-skill")
	if !ok {
		t.Fatal("expected secret-skill")
	}
	if skill.AvailabilityStatus != SkillAvailabilityStatusUnavailable {
		t.Fatalf("expected unavailable executable skill, got %+v", skill)
	}
	if !strings.Contains(skill.AvailabilityReason, "MISSING_SKILL_SECRET") || !strings.Contains(skill.AvailabilityReason, "test") {
		t.Fatalf("expected environment-scoped missing-secret reason, got %q", skill.AvailabilityReason)
	}
}

func TestRegistryReadsExecutableSkillSecretsFromDataDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOPE_ENV", "test")
	t.Setenv("EXEC_SKILL_TOKEN", "prod-shell-secret")
	dataRoot := t.TempDir()
	writeExecutableSkillSecretsFixture(t, dataRoot, map[string]string{"EXEC_SKILL_TOKEN": "test-secret"})
	writeValidExecutableSkillFixture(t, dataRoot, "secret-skill", executableSkillFixtureOptions{
		Description: "needs secret",
		SecretRefs:  []string{"EXEC_SKILL_TOKEN"},
	})

	registry, err := NewRegistry(dataRoot)
	if err != nil {
		t.Fatalf("NewRegistry returned error: %v", err)
	}

	skill, ok := registry.Get("secret-skill")
	if !ok {
		t.Fatal("expected secret-skill")
	}
	if skill.AvailabilityStatus != SkillAvailabilityStatusAvailable {
		t.Fatalf("expected available executable skill, got %+v", skill)
	}
	values, err := ResolveExecutableSkillSecrets(dataRoot, skill.ExecutionManifest.SecretRefs)
	if err != nil {
		t.Fatalf("ResolveExecutableSkillSecrets returned error: %v", err)
	}
	if values["EXEC_SKILL_TOKEN"] != "test-secret" {
		t.Fatalf("expected data-dir scoped secret value, got %+v", values)
	}
}

func TestRegistryKeepsExplicitApprovalOnExecutableSkillFixture(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOPE_ENV", "test")
	dataRoot := t.TempDir()
	writeExecutableSkillSecretsFixture(t, dataRoot, map[string]string{"EXEC_SKILL_TOKEN": "available"})
	writeApprovalGatedExecutableSkillFixture(t, dataRoot, "approval-skill", []string{"EXEC_SKILL_TOKEN"})

	registry, err := NewRegistry(dataRoot)
	if err != nil {
		t.Fatalf("NewRegistry returned error: %v", err)
	}

	skill, ok := registry.Get("approval-skill")
	if !ok {
		t.Fatal("expected approval-skill")
	}
	if skill.ExecutionManifest == nil || skill.ExecutionManifest.ApprovalMode != "ask" {
		t.Fatalf("expected explicit ask approval mode, got %+v", skill)
	}
}

func TestRegistryMarksInvalidExecutableFixtureUnavailable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataRoot := t.TempDir()
	writeInvalidExecutableSkillFixture(t, dataRoot, "invalid-skill", "execution.network_mode: nope")

	registry, err := NewRegistry(dataRoot)
	if err != nil {
		t.Fatalf("NewRegistry returned error: %v", err)
	}

	skill, ok := registry.Get("invalid-skill")
	if !ok {
		t.Fatal("expected invalid-skill")
	}
	if skill.AvailabilityStatus != SkillAvailabilityStatusUnavailable || skill.ExecutionManifest == nil {
		t.Fatalf("expected unavailable invalid executable skill, got %+v", skill)
	}
	if !strings.Contains(skill.AvailabilityReason, "network_mode") {
		t.Fatalf("expected invalid manifest reason, got %q", skill.AvailabilityReason)
	}
}

func writeSkillFixture(t *testing.T, dir, body string) {
	t.Helper()
	writeFileFixture(t, filepath.Join(dir, "SKILL.md"), body)
}

func writeFileFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}

func writeExecutableSkillSecretsFixture(t *testing.T, dataRoot string, values map[string]string) {
	t.Helper()
	payload, err := json.Marshal(values)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	writeFileFixture(t, filepath.Join(dataRoot, executableSkillSecretsFileName), string(payload))
}

type executableSkillFixtureOptions struct {
	Description  string
	ApprovalMode string
	SecretRefs   []string
	Args         []string
	TimeoutMs    int
	ScriptBody   string
}

func writeValidExecutableSkillFixture(t *testing.T, dataRoot, skillID string, opts executableSkillFixtureOptions) {
	t.Helper()
	description := firstExecutableSkillFixtureValue(opts.Description, "executable")
	approvalMode := firstExecutableSkillFixtureValue(opts.ApprovalMode, "ask")
	scriptBody := firstExecutableSkillFixtureValue(opts.ScriptBody, "#!/bin/sh\nprintf ok")

	lines := []string{
		"---",
		"name: " + skillID,
		`description: "` + description + `"`,
		"execution.entrypoint: scripts/run.sh",
		"execution.working_dir: .",
		"execution.profile_id: subprocess_default",
		"execution.read_roots: .",
		"execution.write_roots: .",
		"execution.network_mode: deny",
		"execution.approval_mode: " + approvalMode,
	}
	if len(opts.Args) > 0 {
		lines = append(lines, "execution.args: "+strings.Join(opts.Args, ","))
	}
	if len(opts.SecretRefs) > 0 {
		lines = append(lines, "execution.secret_refs: "+strings.Join(opts.SecretRefs, ","))
	}
	if opts.TimeoutMs > 0 {
		lines = append(lines, "execution.timeout_ms: "+strconv.Itoa(opts.TimeoutMs))
	}
	lines = append(lines, "---", "run it")

	writeSkillFixture(t, filepath.Join(dataRoot, "skills", skillID), strings.Join(lines, "\n"))
	writeFileFixture(t, filepath.Join(dataRoot, "skills", skillID, "scripts", "run.sh"), scriptBody)
}

func writeApprovalGatedExecutableSkillFixture(t *testing.T, dataRoot, skillID string, secretRefs []string) {
	t.Helper()
	writeValidExecutableSkillFixture(t, dataRoot, skillID, executableSkillFixtureOptions{
		ApprovalMode: "ask",
		SecretRefs:   secretRefs,
	})
}

func writeInvalidExecutableSkillFixture(t *testing.T, dataRoot, skillID, invalidManifestLine string) {
	t.Helper()
	writeSkillFixture(t, filepath.Join(dataRoot, "skills", skillID), strings.Join([]string{
		"---",
		"name: " + skillID,
		`description: "invalid executable"`,
		"execution.entrypoint: scripts/run.sh",
		invalidManifestLine,
		"---",
		"run it",
	}, "\n"))
	writeFileFixture(t, filepath.Join(dataRoot, "skills", skillID, "scripts", "run.sh"), "#!/bin/sh\nprintf nope")
}

func firstExecutableSkillFixtureValue(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}
