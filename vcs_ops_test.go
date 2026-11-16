//go:build !integration

package main

import (
	"context"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Jguer/aur"
	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/db/mock"
	"github.com/Jguer/yay/v13/pkg/dep"
	mockaur "github.com/Jguer/yay/v13/pkg/dep/mock"
	"github.com/Jguer/yay/v13/pkg/query"
	"github.com/Jguer/yay/v13/pkg/runtime"
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/settings/exe"
	"github.com/Jguer/yay/v13/pkg/settings/parser"
	"github.com/Jguer/yay/v13/pkg/text"
	"github.com/Jguer/yay/v13/pkg/vcs"
)

func TestInfoToInstallInfo(t *testing.T) {
	info := infoToInstallInfo([]aur.Pkg{
		{Name: "foo", PackageBase: "foo-base"},
		{Name: "bar", PackageBase: "bar-base"},
	})

	require.Len(t, info, 1)
	require.Len(t, info[0], 2)
	require.Equal(t, &dep.InstallInfo{AURBase: "foo-base", Source: dep.AUR}, info[0]["foo"])
	require.Equal(t, &dep.InstallInfo{AURBase: "bar-base", Source: dep.AUR}, info[0]["bar"])
}

func TestCreateDevelDBSerializesPackageUpdates(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")
		store := &develUpdateStore{names: make(chan string, 3)}
		aurClient := &mockaur.MockAUR{GetFn: func(context.Context, *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{Name: "jellyfin", PackageBase: "jfin"}}, nil
		}}
		dbExecutor := &mock.DBExecutor{
			InstalledRemotePackageNamesFn: func() []string { return []string{"jellyfin"} },
			ReposFn:                       func() []string { return nil },
			SyncPackagesFn:                func(...string) []mock.IPackage { return nil },
		}
		run := &runtime.Runtime{
			Cfg:        &settings.Configuration{BuildDir: "testdata", ReDownload: "all"},
			CmdBuilder: &exe.MockBuilder{Runner: &exe.MockRunner{}},
			Logger:     logger,
			AURClient:  aurClient,
			VCSStore:   store,
			QueryBuilder: query.NewSourceQueryBuilder(nil, logger, "", parser.ModeRepo,
				"", false, false, false),
		}

		require.NoError(t, createDevelDB(t.Context(), run, dbExecutor))
		require.False(t, store.overlap.Load(), "package updates must not race with another package's save")
		close(store.names)
		var names []string
		for name := range store.names {
			names = append(names, name)
		}
		require.ElementsMatch(t, []string{"jellyfin", "jellyfin-web", "jellyfin-server"}, names)
	})
}

type develUpdateStore struct {
	vcs.Store
	updating atomic.Bool
	overlap  atomic.Bool
	names    chan string
}

func (s *develUpdateStore) Update(_ context.Context, name string, _ []gosrc.ArchString) {
	if s.updating.Swap(true) {
		s.overlap.Store(true)
	}
	time.Sleep(time.Second)
	s.names <- name
	s.updating.Store(false)
}
