package settings

import (
	"fmt"
	"path/filepath"
	"strings"
)

// DefaultPkgbuildRepoDepth is the recursive PKGBUILD scan depth used when a
// repo does not set one explicitly.
const DefaultPkgbuildRepoDepth = 3

// PkgbuildRepo is a user-configured PKGBUILD repository. Packages found in a
// PKGBUILD repository take priority over the AUR, so a repo can mask an AUR
// package. Repositories are configured only through init.lua via
// yay.opt.pkgbuild_repos.
type PkgbuildRepo struct {
	// Name identifies the repo; it comes from the pkgbuild_repos table key.
	Name string `json:"name" lua:"name"`
	// URL locates the repo, following the makepkg source convention: an https
	// git URL, a git+file:// local git repo, or a file:// local directory used
	// in place.
	URL string `json:"url" lua:"url"`
	// Depth is how many directory levels deep yay scans for PKGBUILDs.
	Depth int `json:"depth" lua:"depth"`
}

// NormalizePkgbuildRepos validates repository names and fills in default depths.
func (c *Configuration) NormalizePkgbuildRepos() error {
	for i := range c.PkgbuildRepos {
		name := c.PkgbuildRepos[i].Name
		if name == "" || strings.HasPrefix(name, "-") || !filepath.IsLocal(name) || filepath.Base(name) != name {
			return fmt.Errorf("invalid PKGBUILD repository name %q", name)
		}

		if c.PkgbuildRepos[i].Depth <= 0 {
			c.PkgbuildRepos[i].Depth = DefaultPkgbuildRepoDepth
		}
	}

	return nil
}
