// Package content resolves the vendored content catalog: design systems,
// skills and design templates.
//
// Everything here is read-only and re-scanned on demand, so dropping a new
// design-system folder into content/design-systems is picked up without a
// daemon restart.
package content

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/akki/pidesigner/internal/model"
	"gopkg.in/yaml.v3"
)

// Catalog indexes the content directories.
type Catalog struct {
	Root string

	mu           sync.RWMutex
	systems      []model.DesignSystem
	systemsByID  map[string]model.DesignSystem
	skills       []model.Skill
	skillsByID   map[string]model.Skill
	templates    []Template
	templatesByK map[string]Template
}

// Template is a renderable design template (a blueprint for an artifact).
type Template struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Mode        string   `json:"mode,omitempty"`
	Scenario    string   `json:"scenario,omitempty"`
	Path        string   `json:"-"`
	Triggers    []string `json:"triggers,omitempty"`
}

// Open builds a catalog rooted at the given content directory. The root is
// made absolute: the agent runs inside a project directory, so every path
// the catalog hands it must resolve from anywhere.
func Open(root string) (*Catalog, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	c := &Catalog{
		Root:         abs,
		systemsByID:  map[string]model.DesignSystem{},
		skillsByID:   map[string]model.Skill{},
		templatesByK: map[string]Template{},
	}
	c.Scan()
	return c, nil
}

// Scan re-reads the content directories.
func (c *Catalog) Scan() {
	systems, systemsByID := c.scanDesignSystems()
	skills, skillsByID := c.scanSkills()
	templates, templatesByK := c.scanTemplates()

	c.mu.Lock()
	c.systems, c.systemsByID = systems, systemsByID
	c.skills, c.skillsByID = skills, skillsByID
	c.templates, c.templatesByK = templates, templatesByK
	c.mu.Unlock()
}

func (c *Catalog) scanDesignSystems() ([]model.DesignSystem, map[string]model.DesignSystem) {
	base := filepath.Join(c.Root, "design-systems")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, map[string]model.DesignSystem{}
	}
	out := []model.DesignSystem{}
	byID := map[string]model.DesignSystem{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		dir := filepath.Join(base, e.Name())
		ds := model.DesignSystem{
			ID:   e.Name(),
			Slug: e.Name(),
			Name: e.Name(),
		}
		if raw, err := os.ReadFile(filepath.Join(dir, "manifest.json")); err == nil {
			var m struct {
				ID          string `yaml:"id"`
				Name        string `yaml:"name"`
				Category    string `yaml:"category"`
				Description string `yaml:"description"`
				Source      struct {
					Type   string `yaml:"type"`
					Origin string `yaml:"origin"`
				} `yaml:"source"`
				Files map[string]string `yaml:"files"`
			}
			if yaml.Unmarshal(raw, &m) == nil {
				if m.ID != "" {
					ds.ID = m.ID
				}
				if m.Name != "" {
					ds.Name = m.Name
				}
				ds.Category = m.Category
				ds.Description = m.Description
				ds.Source = m.Source.Origin
				if ds.Source == "" {
					ds.Source = m.Source.Type
				}
				for k, v := range m.Files {
					ds.Files = append(ds.Files, v)
					_ = k
				}
			}
		}
		ds.HasPreview = dirExists(filepath.Join(dir, "preview"))
		sort.Strings(ds.Files)
		out = append(out, ds)
		byID[ds.ID] = ds
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Name < out[j].Name
	})
	return out, byID
}

// DesignSystemContext is the agent-facing payload for one design system.
type DesignSystemContext struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DesignMD    string `json:"designMd,omitempty"`
	TokensCSS   string `json:"tokensCss,omitempty"`
	UsageMD     string `json:"usageMd,omitempty"`
	TokensJSON  string `json:"tokensJson,omitempty"`
	ComponentHx string `json:"componentsHtml,omitempty"`
}

// DesignSystemContext loads the brand contract for a design system so it can
// be appended to the agent's system prompt.
func (c *Catalog) DesignSystemContext(id string) (*DesignSystemContext, error) {
	if id == "" {
		return nil, nil
	}
	dir := filepath.Join(c.Root, "design-systems", id)
	if !dirExists(dir) {
		return nil, fmt.Errorf("design system %q not found", id)
	}
	ds := &DesignSystemContext{ID: id, Name: id}
	ds.DesignMD = readMaybe(filepath.Join(dir, "DESIGN.md"))
	ds.TokensCSS = readMaybe(filepath.Join(dir, "tokens.css"))
	ds.UsageMD = readMaybe(filepath.Join(dir, "USAGE.md"))
	ds.TokensJSON = readMaybe(filepath.Join(dir, "design-tokens.json"))
	ds.ComponentHx = readMaybe(filepath.Join(dir, "components.html"))
	if ds.DesignMD == "" {
		return nil, fmt.Errorf("design system %q has no DESIGN.md", id)
	}
	return ds, nil
}

// DesignSystems lists the catalog.
func (c *Catalog) DesignSystems() []model.DesignSystem {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]model.DesignSystem(nil), c.systems...)
}

// DesignSystem looks one up by id or slug.
func (c *Catalog) DesignSystem(id string) (model.DesignSystem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ds, ok := c.systemsByID[id]
	return ds, ok
}

// SkillFrontmatter is the parsed header of a SKILL.md.
type SkillFrontmatter struct {
	Name        string   `yaml:"name"`
	EnName      string   `yaml:"en_name"`
	ZhName      string   `yaml:"zh_name"`
	Description string   `yaml:"description"`
	Triggers    []string `yaml:"triggers"`
	OD          struct {
		Mode     string `yaml:"mode"`
		Scenario string `yaml:"scenario"`
		Surface  string `yaml:"surface"`
		Preview  struct {
			Type  string `yaml:"type"`
			Entry string `yaml:"entry"`
		} `yaml:"preview"`
	} `yaml:"od"`
}

func (c *Catalog) scanSkills() ([]model.Skill, map[string]model.Skill) {
	base := filepath.Join(c.Root, "skills")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, map[string]model.Skill{}
	}
	out := []model.Skill{}
	byID := map[string]model.Skill{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(base, e.Name())
		fm, err := ParseSkillFrontmatter(filepath.Join(dir, "SKILL.md"))
		if err != nil {
			continue
		}
		s := model.Skill{
			ID:          e.Name(),
			Name:        fm.Name,
			Description: strings.TrimSpace(fm.Description),
			Triggers:    fm.Triggers,
			Mode:        fm.OD.Mode,
			Scenario:    fm.OD.Scenario,
			Path:        dir,
			HasScripts:  dirExists(filepath.Join(dir, "scripts")),
		}
		if s.Name == "" {
			s.Name = e.Name()
		}
		out = append(out, s)
		byID[s.ID] = s
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, byID
}

// Skills lists every runnable SKILL.md, including design-template workflows.
// This is the same catalog Skill resolves for generation and document reads.
func (c *Catalog) Skills() []model.Skill {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := append([]model.Skill(nil), c.skills...)
	for _, t := range c.templates {
		if _, duplicate := c.skillsByID[t.ID]; duplicate {
			continue
		}
		if _, runnable := c.templatesByK[t.ID]; !runnable {
			continue
		}
		out = append(out, templateSkill(t))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Skill looks up any runnable workflow by id.
func (c *Catalog) Skill(id string) (model.Skill, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if s, ok := c.skillsByID[id]; ok {
		return s, true
	}
	if t, ok := c.templatesByK[id]; ok {
		return templateSkill(t), true
	}
	return model.Skill{}, false
}

func templateSkill(t Template) model.Skill {
	return model.Skill{ID: t.ID, Name: t.Name, Description: t.Description,
		Triggers: t.Triggers, Mode: t.Mode, Scenario: t.Scenario, Path: t.Path,
		HasScripts: dirExists(filepath.Join(t.Path, "scripts"))}
}

func (c *Catalog) scanTemplates() ([]Template, map[string]Template) {
	base := filepath.Join(c.Root, "design-templates")
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, map[string]Template{}
	}
	out := []Template{}
	byID := map[string]Template{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(base, e.Name())
		fm, err := ParseSkillFrontmatter(filepath.Join(dir, "SKILL.md"))
		if err != nil {
			out = append(out, Template{ID: e.Name(), Name: e.Name(), Path: dir})
			continue
		}
		t := Template{
			ID:          e.Name(),
			Name:        fm.Name,
			Description: strings.TrimSpace(fm.Description),
			Mode:        fm.OD.Mode,
			Scenario:    fm.OD.Scenario,
			Path:        dir,
			Triggers:    fm.Triggers,
		}
		out = append(out, t)
		byID[t.ID] = t
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, byID
}

// Templates lists the catalog.
func (c *Catalog) Templates() []Template {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]Template(nil), c.templates...)
}

// Template looks one up by id.
func (c *Catalog) Template(id string) (Template, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, ok := c.templatesByK[id]
	return t, ok
}

// ParseSkillFrontmatter reads the YAML header of a SKILL.md file.
func ParseSkillFrontmatter(path string) (SkillFrontmatter, error) {
	var fm SkillFrontmatter
	raw, err := os.ReadFile(path)
	if err != nil {
		return fm, err
	}
	text := string(raw)
	if !strings.HasPrefix(text, "---\n") {
		return fm, fmt.Errorf("no frontmatter in %s", path)
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return fm, fmt.Errorf("unterminated frontmatter in %s", path)
	}
	body := text[4 : 4+end]
	if err := yaml.Unmarshal([]byte(body), &fm); err != nil {
		return fm, err
	}
	return fm, nil
}

// CraftDoc returns the text of a craft law document (design guidance).
func (c *Catalog) CraftDoc(name string) string {
	return readMaybe(filepath.Join(c.Root, "craft", name))
}

// CraftIndex lists the available craft law documents.
func (c *Catalog) CraftIndex() []string {
	entries, err := os.ReadDir(filepath.Join(c.Root, "craft"))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------- helpers

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func readMaybe(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}
