package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/snestors/agenteshub/internal/cliengine"
	projectfs "github.com/snestors/agenteshub/internal/projects"
	"github.com/snestors/agenteshub/internal/store"
	"github.com/snestors/agenteshub/internal/ws"
)

const proposeTpl = `Sos sdd-propose. Generá un proposal.md para este cambio:

**Proyecto**: {{project_name}}
**Path**: {{project_path}}
**Cambio solicitado**: {{description}}

{{#if feedback}}
**El usuario pidió ajustes**: {{feedback}}
{{/if}}

Estructura del proposal.md:
- ## What
- ## Why
- ## Acceptance criteria (lista)
- ## Out of scope (qué NO se hace)

Devolvé solo el markdown del proposal.md, sin explicación previa ni posterior.`

const designTpl = `Sos sdd-design. Generá un design.md técnico para este cambio OpenSpec ya aprobado en proposal.

**Proyecto**: {{project_name}}
**Path**: {{project_path}}
**Change**: {{change_name}}

## proposal.md aprobado
{{proposal}}

{{#if feedback}}
**El usuario pidió ajustes**: {{feedback}}
{{/if}}

Estructura del design.md:
- ## Context
- ## Decisions
- ## Architecture / files affected
- ## Data model / API changes
- ## Risks and mitigations
- ## Spec deltas to create/update

No ejecutes cambios. Devolvé solo markdown.`

const tasksTpl = `Sos sdd-tasks. Generá un tasks.md implementable para este cambio OpenSpec.

**Proyecto**: {{project_name}}
**Path**: {{project_path}}
**Change**: {{change_name}}

## proposal.md
{{proposal}}

## design.md
{{design}}

{{#if feedback}}
**El usuario pidió ajustes**: {{feedback}}
{{/if}}

Estructura del tasks.md:
- ## Implementation tasks
  - [ ] tareas chicas, verificables y ordenadas
- ## Verification
  - [ ] comandos/checks concretos
- ## Rollback
  - [ ] cómo revertir si falla

No ejecutes cambios. Devolvé solo markdown.`

const applyTpl = `Sos sdd-apply. Ejecutá el cambio OpenSpec siguiendo estrictamente los gates ya aprobados.

**Proyecto**: {{project_name}}
**Path**: {{project_path}}
**Change**: {{change_name}}

## proposal.md aprobado
{{proposal}}

## design.md aprobado
{{design}}

## tasks.md aprobado
{{tasks}}

Reglas:
- Implementá solo las tareas aprobadas.
- Creá/actualizá deltas OpenSpec en openspec/changes/{{change_name}}/specs/<capability>/spec.md con requisitos SHALL y escenarios verificables cuando aplique.
- No archives el change. No actualices RELEASE_NOTES.md.
- Al finalizar, devolvé un resumen markdown breve con archivos modificados y checks corridos.`

const verifyTpl = `Sos sdd-verify. Verificá este cambio OpenSpec luego de apply.

**Proyecto**: {{project_name}}
**Path**: {{project_path}}
**Change**: {{change_name}}

## proposal.md
{{proposal}}

## design.md
{{design}}

## tasks.md
{{tasks}}

Revisá que la implementación cumpla proposal/design/tasks y que los specs/deltas usen SHALL y escenarios testeables.
Devolvé solo markdown con:
- ## Verdict (PASS/WARN/FAIL)
- ## Checks
- ## Findings
- ## Required fixes (si hay)
- ## Archive readiness`

type openspecChangeWire struct {
	ID           int64  `json:"id"`
	ProjectID    int64  `json:"project_id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	State        string `json:"state"`
	CurrentPhase string `json:"current_phase"`
	Feedback     string `json:"feedback,omitempty"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	ArchivedAt   int64  `json:"archived_at,omitempty"`
}

type openspecChangeDetailWire struct {
	Change   openspecChangeWire `json:"change"`
	Proposal string             `json:"proposal,omitempty"`
	Design   string             `json:"design,omitempty"`
	Tasks    string             `json:"tasks,omitempty"`
	Verify   string             `json:"verify,omitempty"`
}

type createOpenSpecChangeReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type approveOpenSpecReq struct {
	DryRun bool `json:"dry_run"`
}

type feedbackOpenSpecReq struct {
	Feedback string `json:"feedback"`
}

type openspecSpecWire struct {
	Capability string `json:"capability"`
	Path       string `json:"path"`
	Content    string `json:"content"`
}

var openspecSlugRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,78}[a-z0-9]$|^[a-z0-9]$`)

func (s *Server) handleOpenSpecChangesList(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}
	if err := projectfs.EnsureOpenSpecLayout(project.Path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	changes, err := s.repos.OpenSpec.ListByProject(r.Context(), project.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]openspecChangeWire, 0, len(changes))
	for _, c := range changes {
		out = append(out, openSpecChangeToWire(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"changes": out})
}

func (s *Server) handleOpenSpecChangesCreate(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}
	var req createOpenSpecChangeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(strings.ToLower(req.Name))
	description := strings.TrimSpace(req.Description)
	if !openspecSlugRE.MatchString(name) {
		http.Error(w, "name must be a slug: lowercase letters, numbers and hyphens", http.StatusBadRequest)
		return
	}
	if description == "" {
		http.Error(w, "description required", http.StatusBadRequest)
		return
	}
	if err := projectfs.EnsureOpenSpecLayout(project.Path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.MkdirAll(projectfs.ChangeDir(project.Path, name), 0o755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	id, err := s.repos.OpenSpec.Create(r.Context(), store.OpenSpecChange{
		ProjectID:    project.ID,
		Name:         name,
		Description:  sql.NullString{String: description, Valid: description != ""},
		State:        "pending_proposal",
		CurrentPhase: "proposal",
	})
	if err != nil {
		if isUniqueConstraint(err) {
			http.Error(w, "openspec change already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	change, err := s.repos.OpenSpec.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"change": openSpecChangeToWire(*change)})
}

func (s *Server) handleOpenSpecChangeGet(w http.ResponseWriter, r *http.Request) {
	project, change, ok := s.openSpecChangeFromRequest(w, r)
	if !ok {
		return
	}
	detail, err := s.openSpecDetail(project, change)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleOpenSpecApprove(w http.ResponseWriter, r *http.Request) {
	project, change, ok := s.openSpecChangeFromRequest(w, r)
	if !ok {
		return
	}
	var req approveOpenSpecReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if strings.TrimSpace(nullString(change.Feedback)) != "" {
		if err := s.regenerateOpenSpecPhase(r.Context(), project, change); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		updated, _ := s.repos.OpenSpec.GetByID(r.Context(), change.ID)
		detail, _ := s.openSpecDetail(project, updated)
		writeJSON(w, http.StatusOK, detail)
		return
	}

	switch change.State {
	case "pending_proposal":
		if err := s.generateOpenSpecArtifact(r.Context(), project, change, "proposal", "proposal.md"); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if err := s.repos.OpenSpec.UpdateState(r.Context(), change.ID, "awaiting_approval_proposal", "proposal"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "awaiting_approval_proposal":
		if err := s.generateOpenSpecArtifact(r.Context(), project, change, "design", "design.md"); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if err := s.repos.OpenSpec.UpdateState(r.Context(), change.ID, "awaiting_approval_design", "design"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "awaiting_approval_design":
		if err := s.generateOpenSpecArtifact(r.Context(), project, change, "tasks", "tasks.md"); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if err := s.repos.OpenSpec.UpdateState(r.Context(), change.ID, "awaiting_approval_tasks", "tasks"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "awaiting_approval_tasks":
		if err := s.repos.OpenSpec.UpdateState(r.Context(), change.ID, "applying", "apply"); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		go s.runOpenSpecApplyAndVerify(project.ID, change.ID, req.DryRun)
	case "awaiting_approval_verify":
		if err := projectfs.ArchiveChange(project.Path, change.Name); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := appendOpenSpecReleaseNote(project.Path, change.Name); err != nil {
			s.log.Warn("append openspec release note", "project", project.Name, "change", change.Name, "err", err)
		}
		if err := s.repos.OpenSpec.Archive(r.Context(), change.ID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "archived", "rejected", "applying":
		http.Error(w, "change is not awaiting this approval", http.StatusConflict)
		return
	default:
		http.Error(w, "unsupported state", http.StatusConflict)
		return
	}
	updated, err := s.repos.OpenSpec.GetByID(r.Context(), change.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.broadcastOpenSpecChange(*updated)
	detail, err := s.openSpecDetail(project, updated)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleOpenSpecReject(w http.ResponseWriter, r *http.Request) {
	project, change, ok := s.openSpecChangeFromRequest(w, r)
	if !ok {
		return
	}
	if err := s.repos.OpenSpec.UpdateState(r.Context(), change.ID, "rejected", change.CurrentPhase); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	updated, _ := s.repos.OpenSpec.GetByID(r.Context(), change.ID)
	s.broadcastOpenSpecChange(*updated)
	detail, _ := s.openSpecDetail(project, updated)
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleOpenSpecFeedback(w http.ResponseWriter, r *http.Request) {
	project, change, ok := s.openSpecChangeFromRequest(w, r)
	if !ok {
		return
	}
	var req feedbackOpenSpecReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	feedback := strings.TrimSpace(req.Feedback)
	if feedback == "" {
		http.Error(w, "feedback required", http.StatusBadRequest)
		return
	}
	if change.State == "applying" || change.State == "archived" || change.State == "rejected" {
		http.Error(w, "feedback not allowed in current state", http.StatusConflict)
		return
	}
	if err := s.repos.OpenSpec.SetFeedback(r.Context(), change.ID, feedback); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	change.Feedback = sql.NullString{String: feedback, Valid: true}
	if err := s.regenerateOpenSpecPhase(r.Context(), project, change); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	updated, _ := s.repos.OpenSpec.GetByID(r.Context(), change.ID)
	s.broadcastOpenSpecChange(*updated)
	detail, _ := s.openSpecDetail(project, updated)
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) handleOpenSpecSpecsList(w http.ResponseWriter, r *http.Request) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return
	}
	if err := projectfs.EnsureOpenSpecLayout(project.Path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	specs, err := listOpenSpecSpecs(project.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"specs": specs})
}

func (s *Server) regenerateOpenSpecPhase(ctx context.Context, project *store.Project, change *store.OpenSpecChange) error {
	phase := change.CurrentPhase
	fileName := phase + ".md"
	if phase == "verify" {
		fileName = "verify.md"
	}
	if phase != "proposal" && phase != "design" && phase != "tasks" && phase != "verify" {
		return fmt.Errorf("cannot regenerate phase %q", phase)
	}
	if err := s.generateOpenSpecArtifact(ctx, project, change, phase, fileName); err != nil {
		return err
	}
	return s.repos.OpenSpec.UpdateState(ctx, change.ID, stateForPhaseApproval(phase), phase)
}

// resolvePhaseModel applies the per-project override (.claude/phase-models.yaml)
// on top of the role default. Phase G of the roadmap.
func resolvePhaseModel(projectPath, phase, defEngine, defModel string) (string, string) {
	cfg, err := projectfs.LoadPhaseModels(projectPath)
	if err != nil || cfg == nil {
		return defEngine, defModel
	}
	return cfg.PhaseEngineModel(phase, defEngine, defModel)
}

// roleForOpenSpecPhase maps a SDD phase to the agent role that should run it.
// See ROADMAP "Decisiones clave" — orchestrators get the heavy thinking,
// executors get the mechanical breakdown, verifiers get the judgment call.
func roleForOpenSpecPhase(phase string) string {
	switch phase {
	case "propose", "design":
		return cliengine.RoleOrchestrator
	case "tasks", "apply":
		return cliengine.RoleExecutor
	case "verify":
		return cliengine.RoleVerifier
	default:
		return cliengine.RoleOrchestrator
	}
}

func (s *Server) generateOpenSpecArtifact(ctx context.Context, project *store.Project, change *store.OpenSpecChange, phase, fileName string) error {
	prompt, err := s.buildOpenSpecPrompt(project, change, phase)
	if err != nil {
		return err
	}
	// Each SDD phase has its own role:
	//   propose / design → orchestrator (opus, planning)
	//   tasks            → executor (codex, mechanical breakdown)
	//   verify           → verifier (sonnet, judgment)
	// apply runs in runOpenSpecApplyAndVerify and uses its own resolution.
	defEngine, defModel := cliengine.RoleDefault(roleForOpenSpecPhase(phase))
	engineName, model := resolvePhaseModel(project.Path, phase, defEngine, defModel)
	// 30 min: a single OpenSpec phase (proposal, design, tasks) can grow long
	// when the agent delegates to Task sub-agents to gather codebase context.
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	res, err := s.engines.Run(runCtx, cliengine.RunOpts{
		Prompt:    prompt,
		Channel:   "project",
		Cwd:       project.Path,
		Engine:    engineName,
		Model:     model,
		Scope:     "agent",
		ProjectID: project.ID,
		AgentName: "sdd-" + phase,
	})
	if err != nil {
		return fmt.Errorf("sdd-%s: %w", phase, err)
	}
	body := cleanMarkdownOnly(res.Text)
	if body == "" {
		return fmt.Errorf("sdd-%s returned empty markdown", phase)
	}
	if err := projectfs.WriteChangeFile(project.Path, change.Name, fileName, body); err != nil {
		return err
	}
	return nil
}

func (s *Server) runOpenSpecApplyAndVerify(projectID, changeID int64, dryRun bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()
	project, err := s.repos.Projects.GetByID(ctx, projectID)
	if err != nil {
		return
	}
	change, err := s.repos.OpenSpec.GetByID(ctx, changeID)
	if err != nil {
		return
	}
	if dryRun {
		_ = projectfs.WriteChangeFile(project.Path, change.Name, "verify.md", "## Verdict\n\nPASS (dry run)\n\n## Checks\n\n- Apply real omitido por `dry_run=true`.\n- Gate de verify simulado para smoke test.\n\n## Findings\n\nSin cambios de código ejecutados.\n\n## Archive readiness\n\nListo para validar el flujo de UI/API; no archivar como cambio real.")
		_ = s.repos.OpenSpec.UpdateState(ctx, change.ID, "awaiting_approval_verify", "verify")
		if updated, err := s.repos.OpenSpec.GetByID(ctx, change.ID); err == nil {
			s.broadcastOpenSpecChange(*updated)
		}
		return
	}
	applyPrompt, err := s.buildOpenSpecPrompt(project, change, "apply")
	if err != nil {
		_ = projectfs.WriteChangeFile(project.Path, change.Name, "verify.md", "## Verdict\n\nFAIL\n\n## Findings\n\n"+err.Error())
		_ = s.repos.OpenSpec.UpdateState(ctx, change.ID, "awaiting_approval_verify", "verify")
		return
	}
	applyDefEngine, applyDefModel := cliengine.RoleDefault(cliengine.RoleExecutor)
	applyEngine, applyModel := resolvePhaseModel(project.Path, "apply", applyDefEngine, applyDefModel)
	applyRes, applyErr := s.engines.Run(ctx, cliengine.RunOpts{
		Prompt:    applyPrompt,
		Channel:   "project",
		Cwd:       project.Path,
		Engine:    applyEngine,
		Model:     applyModel,
		Scope:     "agent",
		ProjectID: project.ID,
		AgentName: "sdd-apply",
	})
	verifyPrompt, err := s.buildOpenSpecPrompt(project, change, "verify")
	if err != nil {
		verifyPrompt = "Sos sdd-verify. Reportá FAIL porque no se pudo construir el prompt de verificación: " + err.Error()
	}
	if applyErr != nil {
		verifyPrompt += "\n\n## Apply error\n" + applyErr.Error()
	} else if applyRes != nil {
		verifyPrompt += "\n\n## Apply result\n" + applyRes.Text
	}
	verifyDefEngine, verifyDefModel := cliengine.RoleDefault(cliengine.RoleVerifier)
	verifyEngine, verifyModel := resolvePhaseModel(project.Path, "verify", verifyDefEngine, verifyDefModel)
	verifyRes, verifyErr := s.engines.Run(ctx, cliengine.RunOpts{
		Prompt:    verifyPrompt,
		Channel:   "project",
		Cwd:       project.Path,
		Engine:    verifyEngine,
		Model:     verifyModel,
		Scope:     "agent",
		ProjectID: project.ID,
		AgentName: "sdd-verify",
	})
	verifyBody := ""
	if verifyErr != nil {
		verifyBody = "## Verdict\n\nFAIL\n\n## Findings\n\nverify error: " + verifyErr.Error()
	} else if verifyRes != nil {
		verifyBody = cleanMarkdownOnly(verifyRes.Text)
	}
	if strings.TrimSpace(verifyBody) == "" {
		verifyBody = "## Verdict\n\nFAIL\n\n## Findings\n\nEmpty verify result."
	}
	_ = projectfs.WriteChangeFile(project.Path, change.Name, "verify.md", verifyBody)
	_ = s.repos.OpenSpec.UpdateState(ctx, change.ID, "awaiting_approval_verify", "verify")
	if updated, err := s.repos.OpenSpec.GetByID(ctx, change.ID); err == nil {
		s.broadcastOpenSpecChange(*updated)
	}
}

func (s *Server) buildOpenSpecPrompt(project *store.Project, change *store.OpenSpecChange, phase string) (string, error) {
	proposal, err := projectfs.ReadChangeFile(project.Path, change.Name, "proposal.md")
	if err != nil {
		return "", err
	}
	design, err := projectfs.ReadChangeFile(project.Path, change.Name, "design.md")
	if err != nil {
		return "", err
	}
	tasks, err := projectfs.ReadChangeFile(project.Path, change.Name, "tasks.md")
	if err != nil {
		return "", err
	}
	vars := map[string]string{
		"project_name": project.Name,
		"project_path": project.Path,
		"change_name":  change.Name,
		"description":  nullString(change.Description),
		"feedback":     nullString(change.Feedback),
		"proposal":     strings.TrimSpace(proposal),
		"design":       strings.TrimSpace(design),
		"tasks":        strings.TrimSpace(tasks),
	}
	tpl := map[string]string{"proposal": proposeTpl, "design": designTpl, "tasks": tasksTpl, "apply": applyTpl, "verify": verifyTpl}[phase]
	if tpl == "" {
		return "", fmt.Errorf("unknown openspec phase %q", phase)
	}
	prompt := renderOpenSpecTemplate(tpl, vars)
	if skill := readSDDWorkflowSkill(); strings.TrimSpace(skill) != "" {
		prompt = "Instrucciones de la skill sdd-workflow:\n\n" + skill + "\n\n---\n\n" + prompt
	}
	return prompt, nil
}

func renderOpenSpecTemplate(tpl string, vars map[string]string) string {
	out := tpl
	for key, value := range vars {
		block := regexp.MustCompile(`(?s)\{\{#if ` + regexp.QuoteMeta(key) + `\}\}(.*?)\{\{/if\}\}`)
		out = block.ReplaceAllStringFunc(out, func(m string) string {
			parts := block.FindStringSubmatch(m)
			if strings.TrimSpace(value) == "" || len(parts) < 2 {
				return ""
			}
			return parts[1]
		})
	}
	for key, value := range vars {
		out = strings.ReplaceAll(out, "{{"+key+"}}", value)
	}
	return strings.TrimSpace(out)
}

func readSDDWorkflowSkill() string {
	candidates := []string{
		filepath.Join(".claude", "skills", "sdd-workflow", "SKILL.md"),
		filepath.Join(os.Getenv("HOME"), ".claude", "skills", "sdd-workflow", "SKILL.md"),
	}
	for _, path := range candidates {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
			return string(raw)
		}
	}
	return ""
}

func (s *Server) openSpecChangeFromRequest(w http.ResponseWriter, r *http.Request) (*store.Project, *store.OpenSpecChange, bool) {
	project, ok := s.projectFromRequest(w, r)
	if !ok {
		return nil, nil, false
	}
	name := strings.TrimSpace(chi.URLParam(r, "name"))
	if name == "" {
		http.Error(w, "change name required", http.StatusBadRequest)
		return nil, nil, false
	}
	change, err := s.repos.OpenSpec.GetByProjectAndName(r.Context(), project.ID, name)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "openspec change not found", http.StatusNotFound)
		return nil, nil, false
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, nil, false
	}
	return project, change, true
}

func (s *Server) openSpecDetail(project *store.Project, change *store.OpenSpecChange) (openspecChangeDetailWire, error) {
	proposal, err := projectfs.ReadChangeFile(project.Path, change.Name, "proposal.md")
	if err != nil {
		return openspecChangeDetailWire{}, err
	}
	design, err := projectfs.ReadChangeFile(project.Path, change.Name, "design.md")
	if err != nil {
		return openspecChangeDetailWire{}, err
	}
	tasks, err := projectfs.ReadChangeFile(project.Path, change.Name, "tasks.md")
	if err != nil {
		return openspecChangeDetailWire{}, err
	}
	verify, err := projectfs.ReadChangeFile(project.Path, change.Name, "verify.md")
	if err != nil {
		return openspecChangeDetailWire{}, err
	}
	return openspecChangeDetailWire{Change: openSpecChangeToWire(*change), Proposal: proposal, Design: design, Tasks: tasks, Verify: verify}, nil
}

func openSpecChangeToWire(c store.OpenSpecChange) openspecChangeWire {
	return openspecChangeWire{
		ID:           c.ID,
		ProjectID:    c.ProjectID,
		Name:         c.Name,
		Description:  nullString(c.Description),
		State:        c.State,
		CurrentPhase: c.CurrentPhase,
		Feedback:     nullString(c.Feedback),
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
		ArchivedAt:   nullInt(c.ArchivedAt),
	}
}

func stateForPhaseApproval(phase string) string {
	switch phase {
	case "proposal":
		return "awaiting_approval_proposal"
	case "design":
		return "awaiting_approval_design"
	case "tasks":
		return "awaiting_approval_tasks"
	case "verify":
		return "awaiting_approval_verify"
	default:
		return "pending_proposal"
	}
}

func cleanMarkdownOnly(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```markdown") || strings.HasPrefix(s, "```md") || strings.HasPrefix(s, "```") {
		lines := strings.Split(s, "\n")
		if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
			lines = lines[1:]
		}
		if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
			lines = lines[:len(lines)-1]
		}
		s = strings.TrimSpace(strings.Join(lines, "\n"))
	}
	return s
}

func listOpenSpecSpecs(projectPath string) ([]openspecSpecWire, error) {
	root := filepath.Join(projectPath, "openspec", "specs")
	out := []openspecSpecWire{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != "spec.md" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		capability := filepath.Dir(rel)
		if capability == "." {
			capability = strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
		}
		out = append(out, openspecSpecWire{Capability: filepath.ToSlash(capability), Path: filepath.ToSlash(filepath.Join("openspec", "specs", rel)), Content: string(raw)})
		return nil
	})
	if os.IsNotExist(err) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Capability < out[j].Capability })
	return out, nil
}

func appendOpenSpecReleaseNote(projectPath, changeName string) error {
	path := filepath.Join(projectPath, "RELEASE_NOTES.md")
	line := fmt.Sprintf("\n- %s — OpenSpec change `%s` archived.\n", time.Now().Format("2006-01-02"), changeName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}

func (s *Server) broadcastOpenSpecChange(change store.OpenSpecChange) {
	if s.hub == nil {
		return
	}
	raw, err := json.Marshal(openSpecChangeToWire(change))
	if err != nil {
		return
	}
	s.hub.Broadcast(ws.Envelope{Type: "openspec_change", Topic: fmt.Sprintf("openspec:%d:%s", change.ProjectID, change.Name), Payload: raw})
	s.hub.Broadcast(ws.Envelope{Type: "openspec_change", Topic: fmt.Sprintf("openspec:%d", change.ProjectID), Payload: raw})
}
