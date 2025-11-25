package download

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sync"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/multierror"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
	"github.com/Jguer/yay/v12/pkg/text"
)

func AURPKGBUILD(httpClient httpRequestDoer, pkgName, aurURL string) ([]byte, error) {
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
	targets []string, aurURL, dest string, force bool,
) (map[string]bool, error) {
	cloned := make(map[string]bool, len(targets))

	var (
		mux  sync.Mutex
		errs multierror.MultiError
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

			newClone, err := AURPKGBUILDRepo(ctx, cmdBuilder, aurURL, target, dest, force)

			mux.Lock()
			progress := len(cloned)
			if err != nil {
				errs.Add(err)
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

	return cloned, errs.Return()
}

// gzipReadCloser wraps a gzip.Reader and the original Closer to ensure
// the underlying resource is closed properly when we're done.
type gzipReadCloser struct {
	*gzip.Reader
	originalCloser io.Closer
}

func (g *gzipReadCloser) Close() error {
	gzErr := g.Reader.Close()
	bodyErr := g.originalCloser.Close()

	if gzErr != nil {
		return gzErr
	}
	return bodyErr
}

// ScannerCloser combines a bufio.Scanner with a Close method
type ScannerCloser struct {
	*bufio.Scanner
	closer io.Closer
}

// Close closes the underlying resource
func (s *ScannerCloser) Close() error {
	if s.closer != nil {
		return s.closer.Close()
	}
	return nil
}

// GetPackageScanner fetches the AUR packages.gz file and returns a scanner for reading its contents.
// This function uses the unified httpRequestDoer interface which provides both Get and Do methods.
// The caller must call Close() on the returned ScannerCloser when done to properly release resources.
func GetPackageScanner(ctx context.Context, client httpRequestDoer, aurURL string) (*ScannerCloser, error) {
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

	// Try to decompress the response body regardless of Content-Encoding header
	var readCloser io.ReadCloser
	gzReader, err := gzip.NewReader(resp.Body)
	if err == nil {
		readCloser = &gzipReadCloser{
			Reader:         gzReader,
			originalCloser: resp.Body,
		}
	} else {
		readCloser = resp.Body
	}

	scanner := bufio.NewScanner(readCloser)

	return &ScannerCloser{
		Scanner: scanner,
		closer:  readCloser,
	}, nil
}
