package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/settings/exe"
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

func getRepoURL(db string) (string, error) {
	switch db {
	case "core", "extra", "testing":
		return ABSPackageURL, nil
	case "community", "multilib", "community-testing", "multilib-testing":
		return ABSCommunityURL, nil
	}

	return "", ErrInvalidRepository
}

// Return format for pkgbuild
// https://github.com/archlinux/svntogit-community/raw/packages/neovim/trunk/PKGBUILD
func getPackageURL(db, pkgName string) (string, error) {
	repoURL, err := getRepoURL(db)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(_urlPackagePath, repoURL, pkgName), err
}

// Return format for pkgbuild repo
// https://github.com/archlinux/svntogit-community.git
func getPackageRepoURL(db string) (string, error) {
	repoURL, err := getRepoURL(db)
	if err != nil {
		return "", err
	}

	return repoURL + ".git", err
}

// ABSPKGBUILD retrieves the PKGBUILD file to a dest directory.
func ABSPKGBUILD(httpClient httpRequestDoer, dbName, pkgName string) ([]byte, error) {
	packageURL, err := getPackageURL(dbName, pkgName)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Get(packageURL)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, ErrABSPackageNotFound
	}

	defer resp.Body.Close()

	pkgBuild, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return pkgBuild, nil
}

// ABSPKGBUILDRepo retrieves the PKGBUILD repository to a dest directory.
func ABSPKGBUILDRepo(ctx context.Context, cmdBuilder exe.GitCmdBuilder,
	dbName, pkgName, dest string, force bool) (bool, error) {
	pkgURL, err := getPackageRepoURL(dbName)
	if err != nil {
		return false, err
	}

	return downloadGitRepo(ctx, cmdBuilder, pkgURL,
		pkgName, dest, force, "--single-branch", "-b", "packages/"+pkgName)
}
