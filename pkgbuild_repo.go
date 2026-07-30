package main

import (
	"context"
	"path/filepath"

	"github.com/Jguer/yay/v13/pkg/pkgbuildrepo"
	"github.com/Jguer/yay/v13/pkg/runtime"
)

// pkgbuildReposCacheDir is the subdirectory of BuildDir where remote PKGBUILD
// repositories are cloned.
const pkgbuildReposCacheDir = ".pkgbuild-repos"

// loadPkgbuildRepoIndex builds the index of configured PKGBUILD repositories,
// refreshing git repos when refresh is set. It returns nil when no repos are
// configured.
func loadPkgbuildRepoIndex(ctx context.Context, run *runtime.Runtime, refresh bool) (*pkgbuildrepo.Index, error) {
	if len(run.Cfg.PkgbuildRepos) == 0 {
		return nil, nil
	}

	repos := make([]pkgbuildrepo.RepoConfig, len(run.Cfg.PkgbuildRepos))
	for i := range run.Cfg.PkgbuildRepos {
		r := run.Cfg.PkgbuildRepos[i]
		repos[i] = pkgbuildrepo.RepoConfig{Name: r.Name, URL: r.URL, Depth: r.Depth}
	}

	cacheDir := filepath.Join(run.Cfg.BuildDir, pkgbuildReposCacheDir)

	return pkgbuildrepo.Load(ctx, run.CmdBuilder, repos, cacheDir, refresh)
}
