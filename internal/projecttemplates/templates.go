// Package projecttemplates loads project templates from
// ~/.claude/project-templates/*.yaml. Each template declares the agents,
// skills, initial services and the seed CLAUDE.md / SPECS.md a brand-new
// project should start with.
package projecttemplates

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Template is the shape on disk of a project template.
type Template struct {
	Name             string             `yaml:"name" json:"name"`
	Description      string             `yaml:"description" json:"description"`
	Stack            map[string]string  `yaml:"stack" json:"stack,omitempty"`
	Agents           []TemplateAgent    `yaml:"agents" json:"agents"`
	Skills           []string           `yaml:"skills" json:"skills"`
	ServicesInitial  []TemplateService  `yaml:"services_initial" json:"services_initial,omitempty"`
	ClaudeMDSeed     string             `yaml:"claude_md_seed" json:"claude_md_seed,omitempty"`
	SpecMDSeed       string             `yaml:"spec_md_seed" json:"spec_md_seed,omitempty"`
	Path             string             `yaml:"-" json:"path"`
}

type TemplateAgent struct {
	Name        string `yaml:"name" json:"name"`
	Role        string `yaml:"role" json:"role"`
	Engine      string `yaml:"engine" json:"engine"`
	Model       string `yaml:"model" json:"model,omitempty"`
	Description string `yaml:"description" json:"description,omitempty"`
}

type TemplateService struct {
	Kind        string `yaml:"kind" json:"kind"`
	Description string `yaml:"description" json:"description,omitempty"`
	Unit        string `yaml:"unit,omitempty" json:"unit,omitempty"`
	Container   string `yaml:"container,omitempty" json:"container,omitempty"`
	Hostname    string `yaml:"hostname,omitempty" json:"hostname,omitempty"`
	Target      string `yaml:"target,omitempty" json:"target,omitempty"`
	Command     string `yaml:"command,omitempty" json:"command,omitempty"`
	Cwd         string `yaml:"cwd,omitempty" json:"cwd,omitempty"`
	HealthURL   string `yaml:"health_url,omitempty" json:"health_url,omitempty"`
	HealthCmd   string `yaml:"health_cmd,omitempty" json:"health_cmd,omitempty"`
	PublicURL   string `yaml:"public_url,omitempty" json:"public_url,omitempty"`
}

var (
	cacheMu  sync.RWMutex
	cache    []Template
	cacheAt  time.Time
	cacheTTL = 30 * time.Second
)

// List returns every template under ~/.claude/project-templates/, parsed.
// Cached for 30 s.
func List() ([]Template, error) {
	cacheMu.RLock()
	if time.Since(cacheAt) < cacheTTL && cache != nil {
		out := cache
		cacheMu.RUnlock()
		return out, nil
	}
	cacheMu.RUnlock()

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".claude", "project-templates")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Template{}, nil
		}
		return nil, err
	}
	out := make([]Template, 0, len(entries))
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(dir, ent.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var tpl Template
		if err := yaml.Unmarshal(raw, &tpl); err != nil {
			continue
		}
		if tpl.Name == "" {
			tpl.Name = strings.TrimSuffix(ent.Name(), ".yaml")
		}
		tpl.Path = path
		out = append(out, tpl)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	cacheMu.Lock()
	cache = out
	cacheAt = time.Now()
	cacheMu.Unlock()
	return out, nil
}

// Get returns a template by name (case-insensitive), or nil if missing.
func Get(name string) (*Template, error) {
	all, err := List()
	if err != nil {
		return nil, err
	}
	for i := range all {
		if strings.EqualFold(all[i].Name, name) {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("template %q not found", name)
}

// Render substitutes `{{name}}` placeholders in a string with `projectName`.
// Used to materialise CLAUDE.md / SPECS.md / services.yaml from the template.
func Render(s, projectName string) string {
	return strings.ReplaceAll(s, "{{name}}", projectName)
}
