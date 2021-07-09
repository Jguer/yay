package download

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/leonelquinteros/gotext"
)

var ErrInvalidRepository = errors.New(gotext.Get("invalid repository"))
var ErrABSPackageNotFound = errors.New(gotext.Get("package not found in repos"))

const MaxConcurrentFetch = 20
const urlPackagePath = "/plain/trunk/PKGBUILD?"

var ABSPackageURL = "https://github.com/archlinux/svntogit-packages.git"
var ABSCommunityURL = "https://github.com/archlinux/svntogit-community.git"

func getPackageURL(db, pkgName string) (string, error) {
	values := url.Values{}
	values.Set("h", "packages/"+pkgName)
	nameEncoded := values.Encode()
	switch db {
	case "core", "extra", "testing":
		return ABSPackageURL + urlPackagePath + nameEncoded, nil
	case "community", "multilib", "community-testing", "multilib-testing":
		return ABSCommunityURL + urlPackagePath + nameEncoded, nil
	}
	return "", ErrInvalidRepository
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
