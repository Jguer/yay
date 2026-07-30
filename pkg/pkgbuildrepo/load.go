package pkgbuildrepo

import (
	"context"
	"fmt"

	"github.com/Jguer/yay/v13/pkg/settings/exe"
)

// RepoConfig is the subset of a configured PKGBUILD repository needed to
// refresh and scan it. Callers build it from settings.PkgbuildRepo.
type RepoConfig struct {
	Name  string
	URL   string
	Depth int
}

// Load refreshes every configured repo and returns an index of the packages
// they provide. Repos are processed in order, so earlier repos mask later ones
// (and all repos mask the AUR downstream).
func Load(ctx context.Context, cmdBuilder exe.ICmdBuilder,
	repos []RepoConfig, cacheDir string, refresh bool,
) (*Index, error) {
	idx := NewIndex()

	for i := range repos {
		repo := repos[i]

		loc, err := Refresh(ctx, cmdBuilder, repo.Name, repo.URL, cacheDir, refresh)
		if err != nil {
			return nil, fmt.Errorf("refreshing pkgbuild repo %q: %w", repo.Name, err)
		}

		dirs, err := Scan(loc.Dir, repo.Depth)
		if err != nil {
			return nil, fmt.Errorf("scanning pkgbuild repo %q: %w", repo.Name, err)
		}

		if err := idx.AddRepo(repo.Name, dirs); err != nil {
			return nil, fmt.Errorf("indexing pkgbuild repo %q: %w", repo.Name, err)
		}
	}

	return idx, nil
}
