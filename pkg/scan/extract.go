package scan

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ExtractionDirName is the scratch directory, inside the build root, where
// downloaded source archives are unpacked for inspection. It is removed after
// the scan and never built.
const ExtractionDirName = ".yay-scan-extract"

const (
	extractMaxArchives = 20
	extractMaxSize     = 512 << 20 // 512 MiB per archive
	extractTimeout     = 90 * time.Second
)

var archiveExtensions = []string{
	".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tbz2", ".tar.xz", ".txz",
	".tar.zst", ".tzst", ".tar.lz", ".zip", ".jar", ".7z", ".rar", ".gz", ".bz2", ".xz",
}

// ExtractSources unpacks downloaded source archives below root so both the
// heuristics and the agent can inspect their contents.
//
// makepkg is deliberately NOT used: its extraction path also runs prepare(),
// which would execute package code before the audit. bsdtar (libarchive, a
// pacman dependency) only reads the archive and rejects path traversal.
// The returned cleanup function removes the scratch directory.
func ExtractSources(ctx context.Context, root string) (cleanup func(), extracted int) {
	scratch := filepath.Join(root, ExtractionDirName)
	cleanup = func() { _ = os.RemoveAll(scratch) }

	entries, err := os.ReadDir(root)
	if err != nil {
		return cleanup, 0
	}
	if _, statErr := os.Stat(scratch); statErr == nil {
		// A leftover scratch dir from an earlier run is not package data.
		_ = os.RemoveAll(scratch)
	}

	count := 0
	for _, entry := range entries {
		if count >= extractMaxArchives {
			break
		}
		if !entry.Type().IsRegular() || !isArchive(entry.Name()) {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil || info.Size() > extractMaxSize {
			continue
		}
		dest := filepath.Join(scratch, fmt.Sprintf("%02d-%s", count, sanitizeName(entry.Name())))
		if mkdirErr := os.MkdirAll(dest, 0o755); mkdirErr != nil {
			continue
		}
		archive := filepath.Join(root, entry.Name())
		extractCtx, cancel := context.WithTimeout(ctx, extractTimeout)
		// Fixed argument vector; no shell, no package-controlled flags.
		cmd := exec.CommandContext(extractCtx, "bsdtar",
			"-xf", archive, "-C", dest, "--no-same-owner", "--no-same-permissions")
		cmd.Stdout = nil
		cmd.Stderr = nil
		if runErr := cmd.Run(); runErr == nil {
			count++
		} else {
			// Not every source is an archive bsdtar accepts; skip, do not fail.
			_ = os.RemoveAll(dest)
		}
		cancel()
	}
	if count == 0 {
		_ = os.RemoveAll(scratch)
	}
	return cleanup, count
}

func isArchive(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range archiveExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, string(filepath.Separator), "_")
	if len(name) > 80 {
		name = name[:80]
	}
	return name
}
