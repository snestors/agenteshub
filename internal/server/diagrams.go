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
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/snestors/agenthub/internal/cliengine"
	"github.com/snestors/agenthub/internal/store"
)

type diagramWire struct {
	ID             int64  `json:"id"`
	ProjectID      int64  `json:"project_id,omitempty"`
	Title          string `json:"title"`
	Prompt         string `json:"prompt,omitempty"`
	Kind           string `json:"kind"`
	Mermaid        string `json:"mermaid,omitempty"`
	MermaidSource  string `json:"mermaid_source,omitempty"`
	HTMLContent    string `json:"html_content,omitempty"`
	ExcalidrawJSON string `json:"excalidraw_json"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

type diagramUpsertReq struct {
	ProjectID      *int64 `json:"project_id"`
	Title          string `json:"title"`
	Prompt         string `json:"prompt"`
	Kind           string `json:"kind"`
	Mermaid        string `json:"mermaid"`
	MermaidSource  string `json:"mermaid_source"`
	HTMLContent    string `json:"html_content"`
	ExcalidrawJSON string `json:"excalidraw_json"`
}

type diagramGenerateReq struct {
	Prompt    string `json:"prompt"`
	ProjectID *int64 `json:"project_id"`
	Type      string `json:"type"`
}

func (s *Server) handleDiagramsList(w http.ResponseWriter, r *http.Request) {
	var projectID *int64
	if raw := strings.TrimSpace(r.URL.Query().Get("project_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			http.Error(w, "bad project_id", http.StatusBadRequest)
			return
		}
		projectID = &id
	}
	diagrams, err := s.repos.Diagrams.List(r.Context(), projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]diagramWire, 0, len(diagrams))
	for _, d := range diagrams {
		out = append(out, diagramToWire(d))
	}
	writeJSON(w, http.StatusOK, map[string]any{"diagrams": out})
}

func (s *Server) handleDiagramsCreate(w http.ResponseWriter, r *http.Request) {
	var req diagramUpsertReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	d, ok := diagramFromReq(w, req)
	if !ok {
		return
	}
	if err := s.validateDiagramProject(r.Context(), d.ProjectID); err != nil {
		diagramProjectError(w, err)
		return
	}
	id, err := s.repos.Diagrams.Create(r.Context(), d)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	created, err := s.repos.Diagrams.Get(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"diagram": diagramToWire(*created)})
}

func (s *Server) handleDiagramGet(w http.ResponseWriter, r *http.Request) {
	d, ok := s.diagramFromRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"diagram": diagramToWire(*d)})
}

func (s *Server) handleDiagramUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := diagramIDFromRequest(w, r)
	if !ok {
		return
	}
	var req diagramUpsertReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	d, ok := diagramFromReq(w, req)
	if !ok {
		return
	}
	d.ID = id
	if err := s.validateDiagramProject(r.Context(), d.ProjectID); err != nil {
		diagramProjectError(w, err)
		return
	}
	if err := s.repos.Diagrams.Update(r.Context(), d); errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "diagram not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	updated, err := s.repos.Diagrams.Get(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"diagram": diagramToWire(*updated)})
}

func (s *Server) handleDiagramDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := diagramIDFromRequest(w, r)
	if !ok {
		return
	}
	if err := s.repos.Diagrams.Delete(r.Context(), id); errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "diagram not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDiagramsGenerate(w http.ResponseWriter, r *http.Request) {
	var req diagramGenerateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" {
		http.Error(w, "prompt required", http.StatusBadRequest)
		return
	}
	if req.Type != "" && !validDiagramType(req.Type) {
		http.Error(w, "type must be flowchart|sequence|c4|erd|mindmap", http.StatusBadRequest)
		return
	}

	cwd := "."
	projectContext := ""
	var projectID int64
	if req.ProjectID != nil && *req.ProjectID > 0 {
		p, err := s.repos.Projects.GetByID(r.Context(), *req.ProjectID)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		projectID = p.ID
		cwd = p.Path
		projectContext = fmt.Sprintf("\nProyecto AgentHub registrado:\n- name: %s\n- path: %s\n- description: %s\n", p.Name, p.Path, nullString(p.Description))
	}

	engineName, model := s.currentEngineModel(r.Context())
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()
	res, err := s.engines.Run(ctx, cliengine.RunOpts{
		Prompt:    buildDiagramGeneratePrompt(req.Prompt, req.Type, projectContext),
		Channel:   "web",
		Cwd:       cwd,
		Engine:    engineName,
		Model:     model,
		Scope:     "agent",
		ProjectID: projectID,
		AgentName: "diagram-architect",
	})
	if err != nil {
		http.Error(w, "diagram engine: "+err.Error(), http.StatusBadGateway)
		return
	}
	mermaid := cleanMermaid(res.Text)
	if mermaid == "" {
		http.Error(w, "empty mermaid", http.StatusBadGateway)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"title":   diagramTitleFromPrompt(req.Prompt),
		"mermaid": mermaid,
	})
}

func (s *Server) currentEngineModel(ctx context.Context) (string, string) {
	engineName := s.cfg.DefaultEngine
	model := s.cfg.DefaultModel
	if v, _ := s.repos.Settings.Get(ctx, "engine"); strings.TrimSpace(v) != "" {
		engineName = strings.TrimSpace(v)
	}
	if v, _ := s.repos.Settings.Get(ctx, "model"); strings.TrimSpace(v) != "" {
		model = strings.TrimSpace(v)
	}
	return engineName, model
}

func buildDiagramGeneratePrompt(userPrompt, typ, projectContext string) string {
	skill := readDiagramArchitectSkill()
	typeHint := "auto"
	if typ != "" {
		typeHint = typ
	}
	return strings.TrimSpace(fmt.Sprintf(`Usá estas instrucciones de la skill diagram-architect como contrato de salida:

%s

Contexto opcional:%s

Pedido del usuario:
%s

Tipo solicitado: %s

Respondé SOLO con Mermaid válido. No uses fences markdown. No agregues explicación.`, skill, projectContext, userPrompt, typeHint))
}

func readDiagramArchitectSkill() string {
	candidates := []string{
		filepath.Join(".claude", "skills", "diagram-architect", "SKILL.md"),
		filepath.Join(os.Getenv("HOME"), ".claude", "skills", "diagram-architect", "SKILL.md"),
	}
	for _, path := range candidates {
		if path == "" {
			continue
		}
		if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
			return string(raw)
		}
	}
	return "Generate concise, valid Mermaid only. Max 25 nodes. Short labels."
}

func cleanMermaid(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
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

func diagramTitleFromPrompt(prompt string) string {
	words := strings.Fields(prompt)
	if len(words) > 7 {
		words = words[:7]
	}
	if len(words) == 0 {
		return "Nuevo diagrama"
	}
	return "Diagrama: " + strings.Join(words, " ")
}

func validDiagramType(t string) bool {
	switch t {
	case "flowchart", "sequence", "c4", "erd", "mindmap":
		return true
	default:
		return false
	}
}

func diagramFromReq(w http.ResponseWriter, req diagramUpsertReq) (store.Diagram, bool) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		http.Error(w, "title required", http.StatusBadRequest)
		return store.Diagram{}, false
	}
	mermaid := strings.TrimSpace(req.MermaidSource)
	if mermaid == "" {
		mermaid = strings.TrimSpace(req.Mermaid)
	}
	html := strings.TrimSpace(req.HTMLContent)
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		// infer: html if html_content is set and no mermaid; else mermaid
		if html != "" && mermaid == "" {
			kind = "html"
		} else {
			kind = "mermaid"
		}
	}
	if kind != "mermaid" && kind != "html" {
		http.Error(w, "kind must be 'mermaid' or 'html'", http.StatusBadRequest)
		return store.Diagram{}, false
	}
	excalidrawJSON := strings.TrimSpace(req.ExcalidrawJSON)
	if excalidrawJSON == "" {
		// Compat with v1 schema (NOT NULL): persist a stub.
		excalidrawJSON = `{"elements":[],"appState":{}}`
	}
	if kind == "mermaid" && mermaid == "" {
		http.Error(w, "mermaid source required for kind=mermaid", http.StatusBadRequest)
		return store.Diagram{}, false
	}
	if kind == "html" && html == "" {
		http.Error(w, "html_content required for kind=html", http.StatusBadRequest)
		return store.Diagram{}, false
	}
	return store.Diagram{
		ProjectID:      sqlIntPtr(req.ProjectID),
		Title:          title,
		Prompt:         sql.NullString{String: strings.TrimSpace(req.Prompt), Valid: strings.TrimSpace(req.Prompt) != ""},
		MermaidSource:  sql.NullString{String: mermaid, Valid: mermaid != ""},
		ExcalidrawJSON: excalidrawJSON,
		Kind:           kind,
		HTMLContent:    sql.NullString{String: html, Valid: html != ""},
	}, true
}

func (s *Server) validateDiagramProject(ctx context.Context, projectID sql.NullInt64) error {
	if !projectID.Valid || projectID.Int64 <= 0 {
		return nil
	}
	_, err := s.repos.Projects.GetByID(ctx, projectID.Int64)
	return err
}

func diagramProjectError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (s *Server) diagramFromRequest(w http.ResponseWriter, r *http.Request) (*store.Diagram, bool) {
	id, ok := diagramIDFromRequest(w, r)
	if !ok {
		return nil, false
	}
	d, err := s.repos.Diagrams.Get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "diagram not found", http.StatusNotFound)
		return nil, false
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, false
	}
	return d, true
}

func diagramIDFromRequest(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "bad diagram id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func diagramToWire(d store.Diagram) diagramWire {
	mermaid := nullString(d.MermaidSource)
	kind := d.Kind
	if kind == "" {
		kind = "mermaid"
	}
	return diagramWire{
		ID:             d.ID,
		ProjectID:      nullInt(d.ProjectID),
		Title:          d.Title,
		Prompt:         nullString(d.Prompt),
		Kind:           kind,
		Mermaid:        mermaid,
		MermaidSource:  mermaid,
		HTMLContent:    nullString(d.HTMLContent),
		ExcalidrawJSON: d.ExcalidrawJSON,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

func sqlIntPtr(v *int64) sql.NullInt64 {
	if v == nil || *v <= 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}
