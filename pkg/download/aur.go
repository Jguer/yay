package download

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/leonelquinteros/gotext"
)

var AURPackageURL = "https://aur.archlinux.org/cgit/aur.git"

var ErrAURPackageNotFound = errors.New(gotext.Get("package not found in AUR"))

func GetAURPkgbuild(httpClient *http.Client, pkgName string) ([]byte, error) {
	values := url.Values{}
	values.Set("h", pkgName)
	pkgURL := AURPackageURL + "/plain/PKGBUILD?" + values.Encode()

	resp, err := httpClient.Get(pkgURL)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, ErrAURPackageNotFound
	}

	defer resp.Body.Close()

	pkgBuild, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return pkgBuild, nil
}
