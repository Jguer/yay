package pkgbuildrepo

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Jguer/yay/v13/pkg/download"
	"github.com/Jguer/yay/v13/pkg/settings/exe"
)

// Location describes where a repository's PKGBUILDs live on disk and how the
// directory is kept up to date.
type Location struct {
	// Dir is the local directory scanned for PKGBUILDs.
	Dir string
	// IsGit reports whether Dir is a git checkout refreshed via clone/pull.
	IsGit bool
	// CloneURL is the git URL to clone/pull when IsGit is true.
	CloneURL string
}

// Resolve maps a repo's configured URL to a Location, following the makepkg
// source convention: remote schemes and git+file:// point at a git checkout in
// cacheDir/name; plain file:// and bare paths are used in place.
func Resolve(name, url, cacheDir string) Location {
	if path, ok := strings.CutPrefix(url, "file://"); ok {
		return Location{Dir: path}
	}

	if isGitURL(url) {
		return Location{
			Dir:      filepath.Join(cacheDir, name),
			IsGit:    true,
			CloneURL: strings.TrimPrefix(url, "git+"),
		}
	}

	return Location{Dir: url}
}

// Refresh resolves the repo location and, for git repos, ensures the clone
// exists in cacheDir. It always clones a missing repo; an existing clone is
// pulled only when refresh is true, so routine installs work offline while
// -Sy keeps repos up to date. Local directory repos are used in place.
func Refresh(ctx context.Context, cmdBuilder exe.GitCmdBuilder,
	name, url, cacheDir string, refresh bool,
) (Location, error) {
	loc := Resolve(name, url, cacheDir)
	if !loc.IsGit {
		return loc, nil
	}

	if !refresh {
		if _, err := os.Stat(filepath.Join(loc.Dir, ".git")); err == nil {
			return loc, nil
		}
	}

	// git clones into cacheDir/name, so the cache dir must exist first.
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return loc, err
	}

	if _, err := download.PkgbuildRepoClone(ctx, cmdBuilder, loc.CloneURL, name, cacheDir, false); err != nil {
		return loc, err
	}

	return loc, nil
}

func isGitURL(url string) bool {
	if strings.HasPrefix(url, "git+") {
		url = strings.TrimPrefix(url, "git+")
	}

	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "git://") {
		return false
	}
	if strings.HasPrefix(url, "file://") {
		return true
	}

	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "ssh://") {
		return true
	}

	if strings.Contains(url, "://") {
		return false
	}

	return strings.HasSuffix(url, ".git")
}
