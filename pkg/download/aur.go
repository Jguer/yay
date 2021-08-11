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

	pkgBuild, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return pkgBuild, nil
}

// AURPkgbuildRepo retrieves the PKGBUILD repository to a dest directory.
func AURPKGBUILDRepo(cmdBuilder exe.GitCmdBuilder, aurURL, pkgName, dest string, force bool) (bool, error) {
	pkgURL := fmt.Sprintf("%s/%s.git", aurURL, pkgName)

	return downloadGitRepo(cmdBuilder, pkgURL, pkgName, dest, force)
}

func AURPKGBUILDRepos(
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
			newClone, err := AURPKGBUILDRepo(cmdBuilder, aurURL, target, dest, force)

			progress := 0

			if err != nil {
				errs.Add(err)
			} else {
				mux.Lock()
				cloned[target] = newClone
				progress = len(cloned)
				mux.Unlock()
			}

			text.OperationInfoln(
				gotext.Get("(%d/%d) Downloaded PKGBUILD: %s",
					progress, len(targets), text.Cyan(target)))

			<-sem

			wg.Done()
		}(target)
	}

	wg.Wait()

	return cloned, errs.Return()
}
