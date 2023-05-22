package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/settings/exe"
)

const (
	MaxConcurrentFetch = 20
	absPackageURL      = "https://gitlab.archlinux.org/archlinux/packaging/packages"
)

var (
	ErrInvalidRepository  = errors.New(gotext.Get("invalid repository"))
	ErrABSPackageNotFound = errors.New(gotext.Get("package not found in repos"))
)

// Return format for pkgbuild
// https://gitlab.archlinux.org/archlinux/packaging/packages/0ad/-/raw/main/PKGBUILD
func getPackagePKGBUILDURL(pkgName string) string {
	return fmt.Sprintf("%s/%s/-/raw/main/PKGBUILD", absPackageURL, pkgName)
}

// Return format for pkgbuild repo
// https://gitlab.archlinux.org/archlinux/packaging/packages/0ad.git
func getPackageRepoURL(pkgName string) string {
	return fmt.Sprintf("%s/%s.git", absPackageURL, pkgName)
}

// ABSPKGBUILD retrieves the PKGBUILD file to a dest directory.
func ABSPKGBUILD(httpClient httpRequestDoer, dbName, pkgName string) ([]byte, error) {
	packageURL := getPackagePKGBUILDURL(pkgName)

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
	dbName, pkgName, dest string, force bool,
) (bool, error) {
	pkgURL := getPackageRepoURL(pkgName)

	return downloadGitRepo(ctx, cmdBuilder, pkgURL,
		pkgName, dest, force, "--single-branch")
}
