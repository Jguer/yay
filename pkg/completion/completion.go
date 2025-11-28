package completion

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/download"
	"github.com/Jguer/yay/v12/pkg/text"
)

type PkgSynchronizer interface {
	SyncPackages(...string) []db.IPackage
}

// Show provides completion info for shells.
func Show(ctx context.Context, httpClient download.HTTPRequestDoer,
	dbExecutor PkgSynchronizer, aurURL, completionPath string, interval int, force bool, logger *text.Logger,
) error {
	err := Update(ctx, httpClient, dbExecutor, aurURL, completionPath, interval, force, logger)
	if err != nil {
		return err
	}

	in, err := os.OpenFile(completionPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer in.Close()

	_, err = io.Copy(os.Stdout, in)

	return err
}

// Update updates completion cache to be used by Complete.
func Update(ctx context.Context, httpClient download.HTTPRequestDoer,
	dbExecutor PkgSynchronizer, aurURL, completionPath string, interval int, force bool, logger *text.Logger,
) error {
	info, err := os.Stat(completionPath)

	if os.IsNotExist(err) || (interval != -1 && time.Since(info.ModTime()).Hours() >= float64(interval*24)) || force {
		errd := os.MkdirAll(filepath.Dir(completionPath), 0o755)
		if errd != nil {
			return errd
		}

		out, errf := os.Create(completionPath)
		if errf != nil {
			return errf
		}

		if createAURList(ctx, httpClient, aurURL, out, logger) != nil {
			defer os.Remove(completionPath)
		}

		erra := createRepoList(dbExecutor, out)

		out.Close()

		return erra
	}

	return nil
}

// createAURList creates a new completion file.
func createAURList(ctx context.Context, client download.HTTPRequestDoer, aurURL string, out io.Writer, logger *text.Logger) error {
	scanner, err := download.GetPackageScanner(ctx, client, aurURL, logger)
	if err != nil {
		return err
	}
	defer scanner.Close()

	scanner.Scan()

	for scanner.Scan() {
		pkgName := scanner.Text()
		if strings.HasPrefix(pkgName, "#") {
			continue
		}

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
