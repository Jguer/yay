package settings

import (
	"fmt"
	"path/filepath"
	"slices"
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

// NormalizePkgbuildRepos validates repository names and URLs, rejects duplicate
// names, and fills in default depths.
func (c *Configuration) NormalizePkgbuildRepos() error {
	seen := make(map[string]struct{}, len(c.PkgbuildRepos))

	for i := range c.PkgbuildRepos {
		name := c.PkgbuildRepos[i].Name
		if name == "" || name == "." || strings.HasPrefix(name, "-") || !filepath.IsLocal(name) || filepath.Base(name) != name {
			return fmt.Errorf("invalid PKGBUILD repository name %q", name)
		}

		// Names become directory names under the cache dir, so two repos sharing
		// one would fight over the same checkout.
		if _, dup := seen[name]; dup {
			return fmt.Errorf("duplicate PKGBUILD repository name %q", name)
		}
		seen[name] = struct{}{}

		if err := validatePkgbuildRepoURL(name, c.PkgbuildRepos[i].URL); err != nil {
			return err
		}

		if c.PkgbuildRepos[i].Depth <= 0 {
			c.PkgbuildRepos[i].Depth = DefaultPkgbuildRepoDepth
		}
	}

	return nil
}

// insecurePkgbuildRepoSchemes are unauthenticated transports. A PKGBUILD repo
// masks the AUR, so fetching one over a tamperable transport would hand an
// on-path attacker arbitrary code execution at build time.
var insecurePkgbuildRepoSchemes = []string{"http://", "git://"}

// PkgbuildRepoGitSchemes are the URL schemes a PKGBUILD repository is cloned
// from (after any "git+" prefix is stripped). Anything else is a local
// directory used in place. Validation and location resolution share this table
// so a scheme can never pass one and be reinterpreted by the other.
var PkgbuildRepoGitSchemes = []string{"https://", "ssh://", "file://"}

// validatePkgbuildRepoURL rejects URLs that Resolve would otherwise silently
// reinterpret as a local filesystem path.
func validatePkgbuildRepoURL(name, url string) error {
	if url == "" {
		return fmt.Errorf("PKGBUILD repository %q has no url", name)
	}

	// The URL is passed to git as a positional argument.
	if strings.HasPrefix(url, "-") {
		return fmt.Errorf("PKGBUILD repository %q has an invalid url %q", name, url)
	}

	bare := strings.TrimPrefix(url, "git+")
	for _, scheme := range insecurePkgbuildRepoSchemes {
		if strings.HasPrefix(bare, scheme) {
			return fmt.Errorf("PKGBUILD repository %q uses insecure protocol %s in %q",
				name, strings.TrimSuffix(scheme, "://"), url)
		}
	}

	if scheme, _, ok := strings.Cut(bare, "://"); ok {
		if slices.Contains(PkgbuildRepoGitSchemes, scheme+"://") {
			return nil
		}

		return fmt.Errorf("PKGBUILD repository %q uses unsupported protocol %s in %q", name, scheme, url)
	}

	// A bare path is used in place (or cloned from, when it ends in .git).
	// Requiring it to be absolute stops a repo from resolving against whatever
	// directory yay happens to be run from.
	if !filepath.IsAbs(url) && !strings.Contains(url, ":") {
		return fmt.Errorf("PKGBUILD repository %q url %q must be absolute", name, url)
	}

	return nil
}
