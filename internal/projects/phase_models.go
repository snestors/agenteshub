package projects

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// PhaseModelsConfig is the optional per-project override file at
// <project>/.claude/phase-models.yaml. When present, its entries trump the
// global RoleDefault() resolver. Missing fields fall back through the role
// default chain.
//
// Example:
//
//	phases:
//	  propose: { engine: claude, model: opus }
//	  design:  { engine: claude, model: opus }
//	  tasks:   { engine: codex,  model: gpt-5.5 }
//	  apply:   { engine: codex,  model: gpt-5.5 }
//	  verify:  { engine: claude, model: sonnet }
//
// Phase G of the roadmap.
type PhaseModelsConfig struct {
	Phases map[string]PhaseModel `yaml:"phases"`
}

type PhaseModel struct {
	Engine string `yaml:"engine"`
	Model  string `yaml:"model"`
}

// LoadPhaseModels reads <projectPath>/.claude/phase-models.yaml. Returns nil
// (no error) when the file is missing — that's the common case and the
// caller falls back to RoleDefault.
func LoadPhaseModels(projectPath string) (*PhaseModelsConfig, error) {
	if projectPath == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(filepath.Join(projectPath, ".claude", "phase-models.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cfg PhaseModelsConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// PhaseEngineModel returns the (engine, model) for a phase, falling back to
// the default tuple when the project doesn't override that phase. Pass
// `defEngine`/`defModel` from `cliengine.RoleDefault(role)`.
func (c *PhaseModelsConfig) PhaseEngineModel(phase, defEngine, defModel string) (string, string) {
	if c == nil || c.Phases == nil {
		return defEngine, defModel
	}
	pm, ok := c.Phases[phase]
	if !ok {
		return defEngine, defModel
	}
	engine := pm.Engine
	model := pm.Model
	if engine == "" {
		engine = defEngine
	}
	if model == "" {
		model = defModel
	}
	return engine, model
}
