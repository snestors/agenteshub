package projects

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureCanon makes sure that <project_path> has the canonical agenthub
// scaffold: CLAUDE.md, SPECS.md, DESIGN.md, AGENTS.md, RELEASE_NOTES.md
// and .agenthub/services.yaml. Existing files are NEVER overwritten —
// missing ones get a minimal skeleton with the project's name + path.
func EnsureCanon(projectPath, projectName, description string) error {
	projectPath = filepath.Clean(strings.TrimSpace(projectPath))
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		projectName = filepath.Base(projectPath)
	}
	description = strings.TrimSpace(description)

	info, err := os.Stat(projectPath)
	if err != nil {
		return fmt.Errorf("stat project path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("project path is not a directory: %s", projectPath)
	}

	files := map[string]string{
		"CLAUDE.md":        claudeSkeleton(projectName, projectPath),
		"SPECS.md":         specsSkeleton(projectName, projectPath, description),
		"DESIGN.md":        designSkeleton(projectName, projectPath),
		"AGENTS.md":        agentsSkeleton(projectName, projectPath),
		"RELEASE_NOTES.md": releaseNotesSkeleton(projectName, projectPath),
	}
	for name, body := range files {
		if err := writeIfMissing(filepath.Join(projectPath, name), body); err != nil {
			return err
		}
	}

	// Claude Code discovers project-local agent context under .claude/.
	// Keep this prompt in sync at creation time, without overwriting user edits.
	if err := os.MkdirAll(filepath.Join(projectPath, ".claude"), 0o755); err != nil {
		return fmt.Errorf("mkdir .claude: %w", err)
	}
	if err := writeIfMissing(filepath.Join(projectPath, ".claude", "CLAUDE.md"), claudeSkeleton(projectName, projectPath)); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(projectPath, ".agenthub"), 0o755); err != nil {
		return fmt.Errorf("mkdir .agenthub: %w", err)
	}
	if err := writeIfMissing(filepath.Join(projectPath, ".agenthub", "services.yaml"), servicesSkeleton(projectName)); err != nil {
		return err
	}
	if err := EnsureOpenSpecLayout(projectPath); err != nil {
		return err
	}
	return nil
}

func writeIfMissing(path, body string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func claudeSkeleton(name, path string) string {
	return fmt.Sprintf(`# %s — Agente del proyecto

Sos el agente de **%s**. Path: %s.

## Stack

(TODO: completar — qué lenguajes, frameworks, deploy target)

## Servicios

(TODO: ver %s para qué corre)

## Convenciones del proyecto

(TODO: qué reglas duras tiene este proyecto. Ej. "tests siempre antes de
commitear", "no tocar /legacy", "deploy se hace con make deploy-prod")

## Decisiones recientes

(TODO: ver DESIGN.md para arquitectura, RELEASE_NOTES.md para historial)
`, name, name, inlineCode(path), inlineCode(".agenthub/services.yaml"))
}

func specsSkeleton(name, path, description string) string {
	desc := description
	if desc == "" {
		desc = "(TODO: describir qué problema resuelve y para quién)"
	}
	return fmt.Sprintf(`# %s — Specs

Path: %s

## Producto

%s

## Capacidades

- TODO: listar capacidades actuales.
- TODO: agregar requisitos verificables y escenarios.

## Fuera de scope

- TODO: documentar límites explícitos.
`, name, inlineCode(path), desc)
}

func designSkeleton(name, path string) string {
	return fmt.Sprintf(`# %s — Diseño

Path: %s

## Arquitectura

- TODO: describir componentes principales y dependencias.

## Decisiones

- TODO: registrar decisiones técnicas con fecha, motivo y tradeoffs.

## Deploy / Operación

- TODO: explicar cómo se build/deploya y qué servicios existen.
`, name, inlineCode(path))
}

func agentsSkeleton(name, path string) string {
	return fmt.Sprintf(`# %s — Agentes

Path: %s

## Agente principal

- TODO: definir rol, límites y modelo preferido.

## Especialistas del proyecto

- TODO: listar agentes custom en .claude/agents/ y cuándo usarlos.

## Skills del proyecto

- TODO: listar skills custom en .claude/skills/ y convenciones heredadas.
`, name, inlineCode(path))
}

func releaseNotesSkeleton(name, path string) string {
	return fmt.Sprintf(`# %s — Release notes

Path: %s

## Unreleased

- TODO: registrar cambios relevantes antes de cada deploy/release.
`, name, inlineCode(path))
}

func servicesSkeleton(name string) string {
	return fmt.Sprintf(`# AgentHub service manifest for %s.
# Declarar acá los servicios que pertenecen al proyecto.
#
# Ejemplos:
# services:
#   - kind: systemd
#     unit: %s.service
#     description: Backend API
#     health_url: http://localhost:8080/health
#     public_url: https://api.example.com
#   - kind: docker
#     container: %s-postgres
#     description: Postgres
#     health_cmd: docker exec %s-postgres pg_isready
#   - kind: cloudflare-tunnel
#     hostname: app.example.com
#     target: http://localhost:3000
#     description: Frontend público
#   - kind: process
#     command: pnpm run dev
#     cwd: ./frontend
#     description: Dev server

services: []
`, name, safeName(name), safeName(name), safeName(name))
}

func safeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, " ", "-")
	return name
}

func inlineCode(s string) string {
	return "`" + s + "`"
}
