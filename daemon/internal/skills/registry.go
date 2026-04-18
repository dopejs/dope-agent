package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
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
	SkillID         string            `json:"skillId"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Source          Source            `json:"source"`
	RootPath        string            `json:"rootPath"`
	SkillPath       string            `json:"skillPath"`
	InstructionPath string            `json:"instructionPath"`
	Frontmatter     map[string]string `json:"frontmatter"`
	FrontmatterRaw  string            `json:"frontmatterRaw,omitempty"`
	Body            string            `json:"body,omitempty"`
	Files           []File            `json:"files"`
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

	homeSkills, err := scanSkills(filepath.Join(r.homeRoot, "skills"), SourceHome)
	if err != nil {
		return err
	}
	dataSkills, err := scanSkills(filepath.Join(r.dataRoot, "skills"), SourceDataDir)
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

func scanSkills(root string, source Source) ([]Skill, error) {
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
		skill, err := loadSkill(filepath.Join(root, entry.Name()), source)
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

func loadSkill(skillRoot string, source Source) (Skill, error) {
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

	return Skill{
		SkillID:         skillID,
		Name:            strings.TrimSpace(skillName),
		Description:     strings.TrimSpace(frontmatter["description"]),
		Source:          source,
		RootPath:        filepath.Dir(skillRoot),
		SkillPath:       skillRoot,
		InstructionPath: instructionPath,
		Frontmatter:     frontmatter,
		FrontmatterRaw:  frontmatterRaw,
		Body:            strings.TrimSpace(body),
		Files:           files,
	}, nil
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
	return cloned
}
