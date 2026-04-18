package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
