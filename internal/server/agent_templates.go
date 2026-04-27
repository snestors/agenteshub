package server

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// AgentTemplate is the parsed frontmatter + body preview of an agent file
// living at ~/.claude/agents/<name>.md. These files are the contract that
// agent-builder uses to materialize mini-agents into the DB; the UI lists
// them as a gallery.
type AgentTemplate struct {
	Name                  string   `yaml:"name" json:"name"`
	Role                  string   `yaml:"role" json:"role"`
	Engine                string   `yaml:"engine" json:"engine,omitempty"`
	Model                 string   `yaml:"model" json:"model,omitempty"`
	Description           string   `yaml:"description" json:"description,omitempty"`
	SkillsRequired        []string `yaml:"skills_required" json:"skills_required,omitempty"`
	SkillsOptional        []string `yaml:"skills_optional" json:"skills_optional,omitempty"`
	Version               string   `yaml:"version" json:"version,omitempty"`
	EngineOverrideReason  string   `yaml:"engine_override_reason" json:"engine_override_reason,omitempty"`
	BodyPreview           string   `yaml:"-" json:"body_preview,omitempty"`
	Path                  string   `yaml:"-" json:"path"`
}

var (
	templatesCacheMu sync.RWMutex
	templatesCache   []AgentTemplate
	templatesCacheAt time.Time
)

const templatesCacheTTL = 30 * time.Second

// handleAgentsTemplates lists every agent template available in
// ~/.claude/agents/, parsed from frontmatter. Cached for 30 s.
func (s *Server) handleAgentsTemplates(w http.ResponseWriter, _ *http.Request) {
	tpls := loadAgentTemplates()
	writeJSON(w, http.StatusOK, map[string]any{"templates": tpls})
}

func loadAgentTemplates() []AgentTemplate {
	templatesCacheMu.RLock()
	if time.Since(templatesCacheAt) < templatesCacheTTL && templatesCache != nil {
		out := templatesCache
		templatesCacheMu.RUnlock()
		return out
	}
	templatesCacheMu.RUnlock()

	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dir := filepath.Join(home, ".claude", "agents")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]AgentTemplate, 0, len(entries))
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".md") {
			continue
		}
		full := filepath.Join(dir, ent.Name())
		raw, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		tpl, ok := parseAgentTemplate(raw, full)
		if !ok {
			continue
		}
		out = append(out, tpl)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	templatesCacheMu.Lock()
	templatesCache = out
	templatesCacheAt = time.Now()
	templatesCacheMu.Unlock()
	return out
}

// parseAgentTemplate extracts YAML frontmatter (between ---/---) and the
// first ~280 chars of body as a preview.
func parseAgentTemplate(raw []byte, path string) (AgentTemplate, bool) {
	s := string(raw)
	if !strings.HasPrefix(s, "---") {
		return AgentTemplate{}, false
	}
	rest := strings.TrimPrefix(s, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return AgentTemplate{}, false
	}
	front := rest[:end]
	body := strings.TrimSpace(rest[end+len("\n---"):])

	var tpl AgentTemplate
	if err := yaml.Unmarshal([]byte(front), &tpl); err != nil {
		return AgentTemplate{}, false
	}
	if tpl.Name == "" {
		return AgentTemplate{}, false
	}
	tpl.Path = path
	preview := body
	if len(preview) > 280 {
		preview = preview[:280] + "…"
	}
	tpl.BodyPreview = preview
	return tpl, true
}
