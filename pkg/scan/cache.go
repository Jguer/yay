package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ScannerVersion invalidates cached reports when rules or the agent change.
// Bump this whenever staticRules, the prompt, or the tool layer changes.
const ScannerVersion = 2

// ContentHash fingerprints everything the scan looks at: relative paths and
// file contents below root, excluding VCS metadata and the extraction scratch
// dir. Any change to the PKGBUILD or the downloaded sources changes the hash.
func ContentHash(root string) (string, error) {
	h := sha256.New()
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (name == ".git" || name == ExtractionDirName) {
				return filepath.SkipDir
			}
			// Top-level src/ and pkg/ are makepkg outputs, not scan inputs;
			// including them would invalidate the cache after every build.
			if filepath.Dir(path) == root && (name == "src" || name == "pkg") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			if strings.HasSuffix(d.Name(), ".part") || strings.Contains(d.Name(), ".pkg.tar.") {
				return nil // partial downloads and built packages are not inputs
			}
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	for _, path := range paths {
		if err := hashFile(h, root, path); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashFile(h hash.Hash, root, path string) error {
	fmt.Fprintf(h, "%s\x00", relativePath(root, path))
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := io.Copy(h, file); err != nil {
		return err
	}
	_, err = h.Write([]byte{0})
	return err
}

// LoadCached returns a previous report for pkg when it is still valid: same
// content, same scanner version, and a complete (error-free) scan. Otherwise
// it returns nil and the package is rescanned.
func LoadCached(reportDir, pkg, hash string) *Report {
	data, err := os.ReadFile(ReportPath(reportDir, pkg))
	if err != nil {
		return nil
	}
	var report Report
	if json.Unmarshal(data, &report) != nil {
		return nil
	}
	if report.ContentHash == "" || report.ContentHash != hash ||
		report.ScannerVersion != ScannerVersion || report.Error != "" {
		return nil
	}
	return &report
}
