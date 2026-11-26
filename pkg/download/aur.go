package download

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v13/pkg/settings/exe"
	"github.com/Jguer/yay/v13/pkg/text"
)

// AURURLResolver returns the ordered AUR base URLs to try for a package name
// or package base.
type AURURLResolver interface {
	URLsFor(name string) []string
}

// StaticAURURLs is an AURURLResolver that returns the same URLs for every
// package.
type StaticAURURLs []string

func (s StaticAURURLs) URLsFor(string) []string { return s }

// aurPKGBUILDFromAny fetches a PKGBUILD from the first URL that has it.
func aurPKGBUILDFromAny(httpClient HTTPRequestDoer, pkgName string, aurURLs []string) ([]byte, error) {
	var errs []error

	for _, aurURL := range aurURLs {
		pkgbuild, err := AURPKGBUILD(httpClient, pkgName, aurURL)
		if err == nil {
			return pkgbuild, nil
		}

		errs = append(errs, err)
	}

	return nil, errors.Join(errs...)
}

// aurPKGBUILDRepoFromAny clones a PKGBUILD repository, trying each URL in
// order until one succeeds. An existing clone is updated from its own remote.
func aurPKGBUILDRepoFromAny(ctx context.Context, cmdBuilder exe.GitCmdBuilder,
	aurURLs []string, pkgName, dest string, force bool,
) (bool, error) {
	if len(aurURLs) == 0 {
		return false, ErrAURPackageNotFound{pkgName: pkgName}
	}

	if _, err := os.Stat(filepath.Join(dest, pkgName, ".git")); err == nil && !force {
		if errO := CheckCloneOrigin(ctx, cmdBuilder, filepath.Join(dest, pkgName), pkgName, aurURLs); errO != nil {
			return false, errO
		}

		return AURPKGBUILDRepo(ctx, cmdBuilder, aurURLs[0], pkgName, dest, force)
	}

	var errs []error

	for _, aurURL := range aurURLs {
		newClone, err := AURPKGBUILDRepo(ctx, cmdBuilder, aurURL, pkgName, dest, force)
		if err == nil {
			return newClone, nil
		}

		errs = append(errs, err)
	}

	return false, errors.Join(errs...)
}

// ErrCloneOriginMismatch is returned when an existing PKGBUILD clone was made
// from a different AUR source than the one the package is fetched from.
type ErrCloneOriginMismatch struct {
	Dir, PkgName, Current, Expected string
}

func (e ErrCloneOriginMismatch) Error() string {
	return gotext.Get("%s was cloned from %s but %s is being fetched from %s; remove %s and try again",
		e.Dir, e.Current, e.PkgName, e.Expected, e.Dir)
}

// CheckCloneOrigin makes sure an existing clone in dir of the PKGBUILD
// repository pkgName comes from the first of aurURLs, the source the package
// was resolved from, rather than silently updating it from another AUR
// source. It only applies when several AUR sources are configured; a
// missing clone, or one whose remote cannot be read, is not an error.
func CheckCloneOrigin(ctx context.Context, cmdBuilder exe.GitCmdBuilder,
	dir, pkgName string, aurURLs []string,
) error {
	if len(aurURLs) < 2 {
		return nil
	}

	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return nil
	}

	stdout, _, err := cmdBuilder.Capture(cmdBuilder.BuildGitCmd(ctx, dir, "remote", "get-url", "origin"))
	current := strings.TrimSpace(stdout)

	if err != nil || current == "" {
		return nil
	}

	expected := fmt.Sprintf("%s/%s.git", strings.TrimRight(aurURLs[0], "/"), pkgName)
	if normalizeGitURL(current) == normalizeGitURL(expected) {
		return nil
	}

	return ErrCloneOriginMismatch{Dir: dir, PkgName: pkgName, Current: current, Expected: expected}
}

func normalizeGitURL(u string) string {
	u = strings.TrimRight(u, "/")
	u = strings.TrimSuffix(u, ".git")

	return strings.TrimRight(u, "/")
}

func AURPKGBUILD(httpClient HTTPRequestDoer, pkgName, aurURL string) ([]byte, error) {
	values := url.Values{}
	values.Set("h", pkgName)
	pkgURL := aurURL + "/cgit/aur.git/plain/PKGBUILD?" + values.Encode()

	resp, err := httpClient.Get(pkgURL)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, ErrAURPackageNotFound{pkgName: pkgName}
	}

	defer resp.Body.Close()

	pkgBuild, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return pkgBuild, nil
}

// AURPkgbuildRepo retrieves the PKGBUILD repository to a dest directory.
func AURPKGBUILDRepo(ctx context.Context, cmdBuilder exe.GitCmdBuilder, aurURL, pkgName, dest string, force bool) (bool, error) {
	pkgURL := fmt.Sprintf("%s/%s.git", aurURL, pkgName)

	return downloadGitRepo(ctx, cmdBuilder, pkgURL, pkgName, dest, force)
}

func AURPKGBUILDRepos(
	ctx context.Context,
	cmdBuilder exe.GitCmdBuilder, logger *text.Logger,
	targets []string, aurURLs AURURLResolver, dest string, force bool,
) (map[string]bool, error) {
	cloned := make(map[string]bool, len(targets))

	var (
		mux  sync.Mutex
		errs []error
		wg   sync.WaitGroup
	)

	sem := make(chan uint8, MaxConcurrentFetch)

	for _, target := range targets {
		sem <- 1
		wg.Add(1)

		go func(target string) {
			defer func() {
				<-sem
				wg.Done()
			}()

			newClone, err := aurPKGBUILDRepoFromAny(ctx, cmdBuilder, aurURLs.URLsFor(target), target, dest, force)

			mux.Lock()
			progress := len(cloned)
			if err != nil {
				errs = append(errs, err)
				mux.Unlock()
				logger.OperationInfoln(
					gotext.Get("(%d/%d) Failed to download PKGBUILD: %s",
						progress, len(targets), text.Cyan(target)))
				return
			}

			cloned[target] = newClone
			progress = len(cloned)
			mux.Unlock()

			logger.OperationInfoln(
				gotext.Get("(%d/%d) Downloaded PKGBUILD: %s",
					progress, len(targets), text.Cyan(target)))
		}(target)
	}

	wg.Wait()

	return cloned, errors.Join(errs...)
}

// ScannerCloser combines a bufio.Scanner with a Close method.
type ScannerCloser struct {
	*bufio.Scanner
	closer io.Closer
}

// Close closes the underlying gzip reader if present.
func (s *ScannerCloser) Close() error {
	if s.closer != nil {
		return s.closer.Close()
	}
	return nil
}

// GetPackageScanner fetches the AUR packages.gz file and returns a scanner for reading its contents.
// The caller must call Close() on the returned ScannerCloser when done to properly release resources.
func GetPackageScanner(ctx context.Context, client HTTPRequestDoer, aurURL string, logger *text.Logger) (*ScannerCloser, error) {
	u, err := url.Parse(aurURL)
	if err != nil {
		return nil, err
	}

	u.Path = path.Join(u.Path, "packages.gz")
	packagesURL := u.String()

	resp, err := client.Get(packagesURL)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("invalid status code: %d", resp.StatusCode)
	}

	// Read the entire body to allow trying gzip decompression
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()

	if err != nil {
		return nil, err
	}

	// Try to decompress as gzip; if that fails, use raw body
	var reader io.Reader
	var closer io.Closer

	gzReader, gzErr := gzip.NewReader(bytes.NewReader(body))
	if gzErr == nil {
		reader = gzReader
		closer = gzReader
	} else {
		if logger != nil {
			logger.Debugln("gzip decompression not needed, using raw response body")
		}
		reader = bytes.NewReader(body)
	}

	scanner := bufio.NewScanner(reader)

	return &ScannerCloser{
		Scanner: scanner,
		closer:  closer,
	}, nil
}
