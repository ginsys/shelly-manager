package sync

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// DefaultExportRoot is the export root when neither sync.export_base_dir nor export.output_directory is set.
const DefaultExportRoot = "./data/exports"

// EnsureExportRoot creates the export root (the default one when root is empty). The server calls it once
// at start, so ResolveUnderRoot can check a root that exists.
func EnsureExportRoot(root string) error {
	if root == "" {
		root = DefaultExportRoot
	}
	return os.MkdirAll(root, 0o755)
}

// ResolveUnderRoot returns the absolute path a request's output_path names under the export root, or an error
// wrapping ErrInvalidExportPath when the path leaves the root. An empty root means DefaultExportRoot and an empty
// path means the root itself; a relative path is joined under the root, and an absolute one must already lie
// inside it. The root must exist. Nothing is created.
//
// The check is lexical first (Abs, Rel, IsLocal). Then it follows symlinks: the deepest part of the path that
// exists, resolved, must lie inside the resolved root, so a symlinked directory inside the root cannot lead out.
// The result is the lexical path, as the operator wrote it. The gap between this check and the later write is not
// closed here; only an os.Root write closes it (OpenOutputRoot). The GitOps, SMA, JSON, YAML and backup exports write
// that way.
func ResolveUnderRoot(root, path string) (string, error) {
	if root == "" {
		root = DefaultExportRoot
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("%w: the export directory %q: %v", ErrInvalidExportPath, root, err)
	}
	outside := fmt.Errorf("%w: output_path must be inside the export directory %s", ErrInvalidExportPath, absRoot)
	if !filepath.IsAbs(path) {
		path = filepath.Join(absRoot, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", outside
	}
	rel, err := filepath.Rel(absRoot, abs)
	if err != nil || !filepath.IsLocal(rel) {
		return "", outside
	}
	target := filepath.Join(absRoot, rel)

	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", fmt.Errorf("%w: the export directory %s cannot be used: %v", ErrInvalidExportPath, absRoot, err)
	}
	if fi, statErr := os.Stat(realRoot); statErr != nil || !fi.IsDir() {
		return "", fmt.Errorf("%w: the export directory %s is not a directory", ErrInvalidExportPath, absRoot)
	}
	for p := target; ; {
		real, evalErr := filepath.EvalSymlinks(p)
		if evalErr == nil {
			if realRel, relErr := filepath.Rel(realRoot, real); relErr != nil || !filepath.IsLocal(realRel) {
				return "", outside
			}
			return target, nil
		}
		parent := filepath.Dir(p)
		if !errors.Is(evalErr, fs.ErrNotExist) || parent == p {
			return "", outside
		}
		p = parent
	}
}

// OpenOutputRoot opens the export root with os.Root and returns it with the output directory's path below it.
// outputPath was resolved by ResolveUnderRoot, so it is a subdirectory of the root. Every write after the check
// goes through the returned root, so a link swapped in after the check cannot redirect it.
func OpenOutputRoot(baseDir, outputPath string) (*os.Root, string, error) {
	if baseDir == "" {
		baseDir = DefaultExportRoot
	}
	absRoot, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, "", fmt.Errorf("resolve the export directory: %w", err)
	}
	rel, err := filepath.Rel(absRoot, outputPath)
	if err != nil || !filepath.IsLocal(rel) {
		return nil, "", fmt.Errorf("%w: output_path must be inside the export directory %s", ErrInvalidExportPath, absRoot)
	}
	root, err := os.OpenRoot(absRoot)
	if err != nil {
		return nil, "", fmt.Errorf("open the export directory %s: %w", absRoot, err)
	}
	return root, rel, nil
}
