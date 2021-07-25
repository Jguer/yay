package download

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/leonelquinteros/gotext"
)

const (
	MaxConcurrentFetch = 20
	_urlPackagePath    = "%s/raw/packages/%s/trunk/PKGBUILD"
)

var (
	ErrInvalidRepository  = errors.New(gotext.Get("invalid repository"))
	ErrABSPackageNotFound = errors.New(gotext.Get("package not found in repos"))
	ABSPackageURL         = "https://github.com/archlinux/svntogit-packages"
	ABSCommunityURL       = "https://github.com/archlinux/svntogit-community"
)

// Return format for pkgbuild
// https://github.com/archlinux/svntogit-community/raw/packages/neovim/trunk/PKGBUILD
func getPackageURL(db, pkgName string) (string, error) {
	repoURL := ""
	switch db {
	case "core", "extra", "testing":
		repoURL = ABSPackageURL
	case "community", "multilib", "community-testing", "multilib-testing":
		repoURL = ABSCommunityURL
	default:
		return "", ErrInvalidRepository
	}

	return fmt.Sprintf(_urlPackagePath, repoURL, pkgName), nil
}

func GetABSPkgbuild(httpClient *http.Client, dbName, pkgName string) ([]byte, error) {
	packageURL, err := getPackageURL(dbName, pkgName)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Get(packageURL)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, ErrABSPackageNotFound
	}

	defer resp.Body.Close()

	pkgBuild, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return pkgBuild, nil
}
