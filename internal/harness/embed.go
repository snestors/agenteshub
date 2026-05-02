package harness

import (
	"embed"
	"io/fs"
	"strings"
)

// templateFS is the embedded BettaTech harness template. The on-disk path is
// internal/harness/template/, but Go's embed package excludes files that
// start with a dot or underscore — so the source uses dot_claude/ which we
// rename to .claude/ at scaffold time. Same trick for any other dot-prefixed
// directory if we add one later.
//
//go:embed template
var templateFS embed.FS

// TemplateVersion returns the version string baked into the binary at build
// time (template/template_version.txt). The string is trimmed.
func TemplateVersion() string {
	raw, err := templateFS.ReadFile("template/template_version.txt")
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(raw))
}

// TemplateFiles walks the embedded template tree and returns every regular
// file it contains, with paths relative to the project root (i.e. dot_claude
// already renamed to .claude, and the leading "template/" stripped).
//
// The order is the natural FS walk order — stable for a given binary build.
func TemplateFiles() ([]TemplateEntry, error) {
	var out []TemplateEntry
	err := fs.WalkDir(templateFS, "template", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Skip the version manifest itself — it's a build-time marker, not a
		// file that lands in the project.
		if p == "template/template_version.txt" {
			return nil
		}
		rel := strings.TrimPrefix(p, "template/")
		dest := dotRename(rel)
		raw, rerr := templateFS.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		out = append(out, TemplateEntry{
			RelPath: dest,
			Content: raw,
			Owner:   OwnerOf(dest),
			Mode:    fileModeFor(dest),
		})
		return nil
	})
	return out, err
}

// TemplateEntry is one file from the embedded template, ready to be written
// into a project's repo.
type TemplateEntry struct {
	RelPath string // path inside the project, e.g. ".claude/agents/leader.md"
	Content []byte
	Owner   string // "template-managed" | "template-seed" | "project"
	Mode    uint32 // file mode for chmod (init.sh needs +x)
}

// dotRename rewrites embed-friendly path segments back to their real names.
// Anywhere a path component starts with "dot_", the prefix is replaced by ".".
// This lets us embed .claude/ as dot_claude/ in the source tree (Go's embed
// would otherwise skip it).
func dotRename(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "dot_") {
			parts[i] = "." + strings.TrimPrefix(part, "dot_")
		}
	}
	return strings.Join(parts, "/")
}

// fileModeFor picks a sensible chmod for a freshly scaffolded file. init.sh
// is the only one that needs +x; everything else is 0644.
func fileModeFor(rel string) uint32 {
	if rel == "init.sh" {
		return 0o755
	}
	return 0o644
}
