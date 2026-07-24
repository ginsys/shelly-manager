package sma

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ginsys/shelly-manager/internal/security"
)

type approvedRootPath struct {
	root     *os.Root
	base     string
	relative string
}

func openApprovedRoot(baseDir, userPath, purpose string) (*approvedRootPath, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("approved %s base directory is required", purpose)
	}
	validated, err := security.ValidatePath(baseDir, userPath)
	if err != nil {
		return nil, fmt.Errorf("%s path validation failed: %w", purpose, err)
	}
	absoluteBase, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("resolve approved %s base directory: %w", purpose, err)
	}
	relative, err := filepath.Rel(absoluteBase, validated)
	if err != nil || !filepath.IsLocal(relative) {
		return nil, fmt.Errorf("%s path is outside the approved base directory", purpose)
	}
	root, err := os.OpenRoot(absoluteBase)
	if err != nil {
		return nil, fmt.Errorf("open approved %s base directory: %w", purpose, err)
	}
	return &approvedRootPath{root: root, base: absoluteBase, relative: relative}, nil
}

func openDefaultExportRoot() (*approvedRootPath, error) {
	temporaryBase, err := filepath.Abs(os.TempDir())
	if err != nil {
		return nil, fmt.Errorf("resolve temporary base directory: %w", err)
	}
	root, err := os.OpenRoot(temporaryBase)
	if err != nil {
		return nil, fmt.Errorf("open temporary base directory: %w", err)
	}
	return &approvedRootPath{
		root:     root,
		base:     temporaryBase,
		relative: defaultOutputDirectory,
	}, nil
}

func (path *approvedRootPath) absolute() string {
	if path.relative == "." {
		return path.base
	}
	return filepath.Join(path.base, path.relative)
}

func (path *approvedRootPath) close() error {
	return path.root.Close()
}
