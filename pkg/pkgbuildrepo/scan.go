// Package pkgbuildrepo scans user-configured PKGBUILD repositories and indexes
// the packages they provide so they can mask AUR packages during resolution.
package pkgbuildrepo

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Scan walks root and returns every directory containing a PKGBUILD, up to
// depth directory levels below root (root itself is level 0). Hidden
// directories such as .git are skipped. The result is sorted for determinism.
func Scan(root string, depth int) ([]string, error) {
	var dirs []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if !d.IsDir() {
			return nil
		}

		level := dirLevel(root, path)

		// Skip hidden directories (e.g. .git) and everything beneath them,
		// but never skip root even if its name begins with a dot.
		if level > 0 && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}

		if level > depth {
			return filepath.SkipDir
		}

		if hasPkgbuild(path) {
			dirs = append(dirs, path)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	slices.Sort(dirs)

	return dirs, nil
}

// dirLevel returns how many directory levels path is below root.
func dirLevel(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return 0
	}

	return strings.Count(rel, string(os.PathSeparator)) + 1
}

func hasPkgbuild(dir string) bool {
	info, err := os.Lstat(filepath.Join(dir, "PKGBUILD"))

	return err == nil && !info.IsDir() && info.Mode()&os.ModeSymlink == 0
}
