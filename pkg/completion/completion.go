package completion

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Jguer/yay/v12/pkg/db"
)

type PkgSynchronizer interface {
	SyncPackages(...string) []db.IPackage
}

type httpRequestDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Show provides completion info for shells.
func Show(ctx context.Context, httpClient httpRequestDoer,
	dbExecutor PkgSynchronizer, aurURL, completionPath string, interval int, force bool,
) error {
	err := Update(ctx, httpClient, dbExecutor, aurURL, completionPath, interval, force)
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
func Update(ctx context.Context, httpClient httpRequestDoer,
	dbExecutor PkgSynchronizer, aurURL, completionPath string, interval int, force bool,
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

		if createAURList(ctx, httpClient, aurURL, out) != nil {
			defer os.Remove(completionPath)
		}

		erra := createRepoList(dbExecutor, out)

		out.Close()

		return erra
	}

	return nil
}

// CreateAURList creates a new completion file.
func createAURList(ctx context.Context, client httpRequestDoer, aurURL string, out io.Writer) error {
	u, err := url.Parse(aurURL)
	if err != nil {
		return err
	}

	u.Path = path.Join(u.Path, "packages.gz")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid status code: %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)

	scanner.Scan()

	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, "#") {
			continue
		}

		if _, err := io.WriteString(out, text+"\tAUR\n"); err != nil {
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
