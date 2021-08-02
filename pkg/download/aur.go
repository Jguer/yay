package download

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/multierror"
	"github.com/Jguer/yay/v10/pkg/settings/exe"
	"github.com/Jguer/yay/v10/pkg/text"
)

var AURPackageURL = "https://aur.archlinux.org/cgit/aur.git"

func AURPKGBUILD(httpClient *http.Client, pkgName string) ([]byte, error) {
	values := url.Values{}
	values.Set("h", pkgName)
	pkgURL := AURPackageURL + "/plain/PKGBUILD?" + values.Encode()

	resp, err := httpClient.Get(pkgURL)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, ErrAURPackageNotFound{pkgName: pkgName}
	}

	defer resp.Body.Close()

	pkgBuild, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return pkgBuild, nil
}

// AURPkgbuildRepo retrieves the PKGBUILD repository to a dest directory.
func AURPKGBUILDRepo(cmdRunner exe.Runner, cmdBuilder exe.GitCmdBuilder, aurURL, pkgName, dest string, force bool) error {
	pkgURL := fmt.Sprintf("%s/%s.git", aurURL, pkgName)

	return downloadGitRepo(cmdRunner, cmdBuilder, pkgURL, pkgName, dest, force)
}

func AURPKGBUILDRepos(
	cmdRunner exe.Runner,
	cmdBuilder exe.GitCmdBuilder,
	targets []string, aurURL, dest string, force bool) (map[string]bool, error) {
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
			err := AURPKGBUILDRepo(cmdRunner, cmdBuilder, aurURL, target, dest, force)

			success := err == nil
			if success {
				mux.Lock()
				cloned[target] = success
				mux.Unlock()
			} else {
				errs.Add(err)
			}

			text.OperationInfoln(
				gotext.Get("(%d/%d) Downloaded PKGBUILD: %s",
					len(cloned), len(targets), text.Cyan(target)))

			<-sem

			wg.Done()
		}(target)
	}

	wg.Wait()

	return cloned, errs.Return()
}
