package projects

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var changeFiles = map[string]bool{
	"proposal.md": true,
	"design.md":   true,
	"tasks.md":    true,
	"verify.md":   true,
}

// EnsureOpenSpecLayout creates openspec/{specs,changes,archive}/ if missing.
func EnsureOpenSpecLayout(projectPath string) error {
	root, err := openSpecRoot(projectPath)
	if err != nil {
		return err
	}
	for _, dir := range []string{"specs", "changes", "archive"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			return fmt.Errorf("mkdir openspec/%s: %w", dir, err)
		}
	}
	return nil
}

// ChangeDir returns the absolute path to a change folder.
func ChangeDir(projectPath, changeName string) string {
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		abs = projectPath
	}
	return filepath.Join(abs, "openspec", "changes", cleanName(changeName))
}

// WriteChangeFile writes proposal.md/design.md/tasks.md inside a change folder.
func WriteChangeFile(projectPath, changeName, fileName, content string) error {
	if err := EnsureOpenSpecLayout(projectPath); err != nil {
		return err
	}
	if !validChangeFile(fileName) {
		return fmt.Errorf("invalid change file: %s", fileName)
	}
	dir := ChangeDir(projectPath, changeName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir change dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, fileName), []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", fileName, err)
	}
	return nil
}

// ReadChangeFile reads one of those files; returns "" + nil if missing.
func ReadChangeFile(projectPath, changeName, fileName string) (string, error) {
	if !validChangeFile(fileName) {
		return "", fmt.Errorf("invalid change file: %s", fileName)
	}
	path := filepath.Join(ChangeDir(projectPath, changeName), fileName)
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", fileName, err)
	}
	return string(raw), nil
}

// ArchiveChange moves changes/<name>/ → archive/<name>/ atomically (os.Rename).
// Updates specs/ from the change's specs/ deltas.
func ArchiveChange(projectPath, changeName string) error {
	if err := EnsureOpenSpecLayout(projectPath); err != nil {
		return err
	}
	root, err := openSpecRoot(projectPath)
	if err != nil {
		return err
	}
	name := cleanName(changeName)
	changeDir := filepath.Join(root, "changes", name)
	archiveDir := filepath.Join(root, "archive", name)
	info, err := os.Stat(changeDir)
	if err != nil {
		return fmt.Errorf("stat change dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("change path is not a directory: %s", changeDir)
	}
	if _, err := os.Stat(archiveDir); err == nil {
		return fmt.Errorf("archive already exists: %s", archiveDir)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat archive dir: %w", err)
	}
	if err := applySpecDeltas(filepath.Join(changeDir, "specs"), filepath.Join(root, "specs"), name, time.Now()); err != nil {
		return err
	}
	if err := os.Rename(changeDir, archiveDir); err != nil {
		return fmt.Errorf("rename change to archive: %w", err)
	}
	return nil
}

func openSpecRoot(projectPath string) (string, error) {
	projectPath = filepath.Clean(strings.TrimSpace(projectPath))
	if projectPath == "" || projectPath == "." {
		return "", fmt.Errorf("project path required")
	}
	abs, err := filepath.Abs(projectPath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("stat project path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project path is not a directory: %s", abs)
	}
	return filepath.Join(abs, "openspec"), nil
}

// applySpecDeltas mergea los deltas de un change archivado en las specs vivas del proyecto.
//
// Para cada `<capability>/spec.md` dentro de deltaRoot:
//   - Si el destino NO existe: copia el delta tal cual (primera vez que se ve la capability).
//   - Si el destino EXISTE: appendea una sección con header
//     "## Delta from change: <changeName> (archived YYYY-MM-DD)" seguida del contenido del delta.
//     Esto preserva la spec viva original y deja trazabilidad de qué change agregó qué.
func applySpecDeltas(deltaRoot, specsRoot, changeName string, archivedAt time.Time) error {
	info, err := os.Stat(deltaRoot)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat spec deltas: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("spec deltas path is not a directory: %s", deltaRoot)
	}
	return filepath.WalkDir(deltaRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(deltaRoot, path)
		if err != nil {
			return err
		}
		if filepath.Base(rel) != "spec.md" {
			return nil
		}
		dst := filepath.Join(specsRoot, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return mergeSpecDelta(dst, raw, changeName, archivedAt)
	})
}

// mergeSpecDelta escribe el delta en dst: si el destino existe, appendea con header;
// si no, lo crea con el contenido del delta tal cual.
func mergeSpecDelta(dst string, delta []byte, changeName string, archivedAt time.Time) error {
	_, err := os.Stat(dst)
	if os.IsNotExist(err) {
		return os.WriteFile(dst, delta, 0o644)
	}
	if err != nil {
		return fmt.Errorf("stat spec dst: %w", err)
	}
	header := fmt.Sprintf("\n\n---\n\n## Delta from change: `%s` (archived %s)\n\n",
		changeName, archivedAt.Format("2006-01-02"))
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open spec dst for append: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(header); err != nil {
		return fmt.Errorf("append delta header: %w", err)
	}
	if _, err := f.Write(delta); err != nil {
		return fmt.Errorf("append delta body: %w", err)
	}
	if len(delta) > 0 && delta[len(delta)-1] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return fmt.Errorf("append delta newline: %w", err)
		}
	}
	return nil
}

func validChangeFile(fileName string) bool {
	return changeFiles[filepath.Base(fileName)] && fileName == filepath.Base(fileName)
}

func cleanName(name string) string {
	return filepath.Base(strings.TrimSpace(name))
}
