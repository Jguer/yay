package completion

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Jguer/yay/v13/pkg/db"
	"github.com/Jguer/yay/v13/pkg/download"
	"github.com/Jguer/yay/v13/pkg/text"
)

// NeedsUpdate checks if the completion cache needs to be regenerated.
// Returns true if the file doesn't exist, is older than interval days, is empty, or force is true.
func NeedsUpdate(completionPath string, interval int, force bool) bool {
	if force {
		return true
	}

	info, err := os.Stat(completionPath)
	if os.IsNotExist(err) {
		return true
	}

	// If the file is empty or invalid, regenerate it
	if info.Size() == 0 {
		return true
	}

	if interval != -1 && time.Since(info.ModTime()).Hours() >= float64(interval*24) {
		return true
	}

	return false
}

type PkgSynchronizer interface {
	SyncPackages(...string) []db.IPackage
}

// Show provides completion info for shells.
func Show(ctx context.Context, httpClient download.HTTPRequestDoer,
	dbExecutor PkgSynchronizer, aurURLs []string, completionPath string, interval int, force bool, logger *text.Logger,
) error {
	if NeedsUpdate(completionPath, interval, force) {
		if err := UpdateCache(ctx, httpClient, dbExecutor, aurURLs, completionPath, logger); err != nil {
			return err
		}
	}

	in, err := os.OpenFile(completionPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = io.Copy(os.Stdout, in)

	return err
}

// UpdateCache regenerates the completion cache file unconditionally.
func UpdateCache(ctx context.Context, httpClient download.HTTPRequestDoer,
	dbExecutor PkgSynchronizer, aurURLs []string, completionPath string, logger *text.Logger,
) error {
	if err := os.MkdirAll(filepath.Dir(completionPath), 0o755); err != nil {
		return err
	}

	out, err := os.Create(completionPath)
	if err != nil {
		return err
	}
	defer out.Close()

	seen := make(map[string]bool)

	for i, aurURL := range aurURLs {
		if err := writeAURList(ctx, httpClient, aurURL, out, logger, seen); err != nil {
			// The primary source is required; extra sources are best effort.
			if i == 0 {
				os.Remove(completionPath)
				return err
			}

			if logger != nil {
				logger.Debugln("skipping completion list for", aurURL+":", err)
			}
		}
	}

	return createRepoList(dbExecutor, out)
}

// createAURList creates a new completion file.
func createAURList(ctx context.Context, client download.HTTPRequestDoer, aurURL string, out io.Writer, logger *text.Logger) error {
	return writeAURList(ctx, client, aurURL, out, logger, make(map[string]bool))
}

// writeAURList appends the package names of one AUR source, skipping names
// already present in seen.
func writeAURList(ctx context.Context, client download.HTTPRequestDoer,
	aurURL string, out io.Writer, logger *text.Logger, seen map[string]bool,
) error {
	scanner, err := download.GetPackageScanner(ctx, client, aurURL, logger)
	if err != nil {
		return err
	}
	defer scanner.Close()

	scanner.Scan()

	for scanner.Scan() {
		pkgName := scanner.Text()
		if strings.HasPrefix(pkgName, "#") || seen[pkgName] {
			continue
		}

		seen[pkgName] = true

		if _, err := io.WriteString(out, pkgName+"\tAUR\n"); err != nil {
			return err
		}
	}

	return nil
}

// createRepoList appends Repo packages to completion cache.
func createRepoList(dbExecutor PkgSynchronizer, out io.Writer) error {
	for _, pkg := range dbExecutor.SyncPackages() {
		_, err := io.WriteString(out, pkg.Name()+"\t"+pkg.DB().Name()+"\n")
		if err != nil {
			return err
		}
	}

	return nil
}
