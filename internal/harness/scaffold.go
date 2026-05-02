package harness

import (
	"fmt"
	"os"
	"path/filepath"
)

// ScaffoldResult reports what Scaffold did. The HTTP/MCP layers turn it into
// JSON for the UI to render.
type ScaffoldResult struct {
	TemplateVersion string                 `json:"template_version"`
	Created         []string               `json:"created"`           // files written for the first time
	Skipped         []ScaffoldSkippedEntry `json:"skipped"`           // files that already existed (owner-respect)
	Manifest        *Manifest              `json:"manifest"`          // the manifest now persisted
	AlreadyExisted  bool                   `json:"already_existed"`   // true when .harness/manifest.json was already there
}

// ScaffoldSkippedEntry says why a single file was not written.
type ScaffoldSkippedEntry struct {
	Path   string `json:"path"`
	Owner  string `json:"owner"`
	Reason string `json:"reason"`
}

// Scaffold writes the embedded harness template into projectRoot. Behaviour
// depends on the owner of each file:
//
//   - OwnerTemplateManaged: written; overwrites any local copy.
//   - OwnerTemplateSeed:    written ONLY if the destination doesn't exist.
//   - OwnerProject:         written ONLY if the destination doesn't exist.
//
// Subdirectories are created as needed. If .harness/manifest.json already
// exists, Scaffold treats this as "already scaffolded": it still walks every
// file using the same rules (so a missing project file gets re-seeded), but
// it sets AlreadyExisted=true. To force a full re-scaffold (overwriting
// project-owned files) the caller has to delete .harness/ first.
//
// Manifest is rewritten on success with the current TemplateVersion and a
// fresh sha256 for every file the scaffold touched.
func Scaffold(projectRoot string) (*ScaffoldResult, error) {
	info, err := os.Stat(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("stat project root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("project root is not a directory: %s", projectRoot)
	}

	prev, err := LoadManifest(projectRoot)
	if err != nil {
		return nil, err
	}
	res := &ScaffoldResult{
		TemplateVersion: TemplateVersion(),
		Created:         []string{},
		Skipped:         []ScaffoldSkippedEntry{},
		AlreadyExisted:  prev != nil,
	}

	entries, err := TemplateFiles()
	if err != nil {
		return nil, fmt.Errorf("walk template: %w", err)
	}

	manifestFiles := map[string]ManifestFile{}

	for _, e := range entries {
		dst := filepath.Join(projectRoot, e.RelPath)
		exists := false
		if _, err := os.Stat(dst); err == nil {
			exists = true
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat %s: %w", e.RelPath, err)
		}

		shouldWrite := false
		switch e.Owner {
		case OwnerTemplateManaged:
			shouldWrite = true
		case OwnerTemplateSeed, OwnerProject:
			shouldWrite = !exists
		}

		if !shouldWrite {
			res.Skipped = append(res.Skipped, ScaffoldSkippedEntry{
				Path:   e.RelPath,
				Owner:  e.Owner,
				Reason: "already exists; owner=" + e.Owner,
			})
			// Still record the file in the manifest with the EXISTING content's
			// hash so update-flow knows what's in the project right now.
			if existing, rerr := os.ReadFile(dst); rerr == nil {
				manifestFiles[e.RelPath] = ManifestFile{Owner: e.Owner, SHA256: hashContent(existing)}
			} else {
				manifestFiles[e.RelPath] = ManifestFile{Owner: e.Owner}
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
		}
		if err := os.WriteFile(dst, e.Content, os.FileMode(e.Mode)); err != nil {
			return nil, fmt.Errorf("write %s: %w", e.RelPath, err)
		}
		// Re-chmod separately so umask doesn't strip +x from init.sh.
		_ = os.Chmod(dst, os.FileMode(e.Mode))

		res.Created = append(res.Created, e.RelPath)
		manifestFiles[e.RelPath] = ManifestFile{Owner: e.Owner, SHA256: hashContent(e.Content)}
	}

	now := nowRFC3339()
	manifest := &Manifest{
		TemplateVersionApplied: res.TemplateVersion,
		Files:                  manifestFiles,
	}
	if prev != nil {
		manifest.ScaffoldedAt = prev.ScaffoldedAt
		manifest.LastUpdateAt = now
	} else {
		manifest.ScaffoldedAt = now
	}
	if err := SaveManifest(projectRoot, manifest); err != nil {
		return nil, err
	}
	res.Manifest = manifest

	return res, nil
}
