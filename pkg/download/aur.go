package download

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/Jguer/yay/v10/pkg/settings/exe"
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
