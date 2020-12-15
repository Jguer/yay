package download

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
)

const AURPackageURL = "https://aur.archlinux.org/cgit/aur.git/plain/PKGBUILD?"

var ErrAURPackageNotFound = errors.New("package not found in AUR")

func GetAURPkgbuild(pkgName string) ([]byte, error) {
	values := url.Values{}
	values.Set("h", pkgName)
	pkgURL := AURPackageURL + values.Encode()

	resp, err := http.Get(pkgURL)
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
