package harness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ManifestPath is where each scaffolded project records the harness it has.
const ManifestPath = ".harness/manifest.json"

// Owner classifications. See AGENTS.md in the template for the full
// description. Short version:
//
//   - OwnerTemplateManaged: template wins. Updates overwrite local edits.
//   - OwnerTemplateSeed:    template wins on first scaffold; afterwards the
//                           project owns it. Updates skip these.
//   - OwnerProject:         project always wins. Updates never touch these.
const (
	OwnerTemplateManaged = "template-managed"
	OwnerTemplateSeed    = "template-seed"
	OwnerProject         = "project"
)

// projectOwnedPaths and templateSeedPaths are the deviations from the default
// (which is OwnerTemplateManaged). Add new files here as the template grows.
var (
	projectOwnedPaths = map[string]struct{}{
		"feature_list.json":     {},
		"progress/current.md":   {},
		"progress/history.md":   {},
	}
	templateSeedPaths = map[string]struct{}{
		"AGENTS.md":              {},
		"docs/architecture.md":   {},
		"docs/conventions.md":    {},
	}
)

// OwnerOf returns the owner classification for a path inside the project.
func OwnerOf(rel string) string {
	if _, ok := projectOwnedPaths[rel]; ok {
		return OwnerProject
	}
	if _, ok := templateSeedPaths[rel]; ok {
		return OwnerTemplateSeed
	}
	return OwnerTemplateManaged
}

// Manifest is the on-disk record of which template was scaffolded into a
// project and when. Stored at .harness/manifest.json so the scaffold/update
// flow can detect drift between local files and the embedded template.
type Manifest struct {
	TemplateVersionApplied string                  `json:"template_version_applied"`
	ScaffoldedAt           string                  `json:"scaffolded_at"`
	LastUpdateAt           string                  `json:"last_update_at,omitempty"`
	Files                  map[string]ManifestFile `json:"files"`
}

// ManifestFile is the per-file record inside the manifest.
type ManifestFile struct {
	Owner  string `json:"owner"`
	SHA256 string `json:"sha256,omitempty"` // empty for project-owned files we never hash
}

// LoadManifest reads .harness/manifest.json from a project root, or returns
// (nil, nil) if it doesn't exist. JSON parse errors propagate so the caller
// sees them; a missing file is NOT an error — that's a valid "never
// scaffolded" state.
func LoadManifest(projectRoot string) (*Manifest, error) {
	full := filepath.Join(projectRoot, ManifestPath)
	raw, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}

// SaveManifest writes m to .harness/manifest.json under projectRoot atomically
// (tmp + rename in the same dir). Creates .harness/ if missing.
func SaveManifest(projectRoot string, m *Manifest) error {
	dir := filepath.Join(projectRoot, ".harness")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir .harness: %w", err)
	}
	dst := filepath.Join(projectRoot, ManifestPath)
	enc, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	enc = append(enc, '\n')
	tmp, err := os.CreateTemp(dir, ".manifest-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create manifest tmp: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, err := tmp.Write(enc); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write manifest tmp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync manifest tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close manifest tmp: %w", err)
	}
	_ = os.Chmod(tmpPath, 0o644)
	if err := os.Rename(tmpPath, dst); err != nil {
		return fmt.Errorf("rename manifest: %w", err)
	}
	cleanup = false
	return nil
}

// hashContent returns the hex-encoded sha256 of b.
func hashContent(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// nowRFC3339 returns time.Now() in RFC3339 — kept here so manifest writers
// don't drift between formats.
func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
