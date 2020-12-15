package download

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
)

var ErrInvalidRepository = errors.New("invalid repository")
var ErrABSPackageNotFound = errors.New("package not found in repos")

const MaxConcurrentFetch = 20
const ABSPackageURL = "https://git.archlinux.org/svntogit/packages.git/plain/trunk/PKGBUILD?"
const ABSCommunityURL = "https://git.archlinux.org/svntogit/community.git/plain/trunk/PKGBUILD?"

func getPackageURL(db, pkgName string) (string, error) {
	values := url.Values{}
	values.Set("h", "packages/"+pkgName)
	nameEncoded := values.Encode()
	switch db {
	case "core", "extra", "testing":
		return ABSPackageURL + nameEncoded, nil
	case "community", "multilib", "community-testing", "multilib-testing":
		return ABSCommunityURL + nameEncoded, nil
	}
	return "", ErrInvalidRepository
}

func GetABSPkgbuild(dbName, pkgName string) ([]byte, error) {
	packageURL, err := getPackageURL(dbName, pkgName)
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(packageURL)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	pkgBuild, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return pkgBuild, nil
}
