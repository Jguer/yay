//go:build !integration
// +build !integration

package dep

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"

	aurc "github.com/Jguer/aur"
	alpm "github.com/Jguer/go-alpm/v2"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/db/mock"
	mockaur "github.com/Jguer/yay/v12/pkg/dep/mock"
	aur "github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/text"
)

func ptrString(s string) *string {
	return &s
}

func getFromFile(t *testing.T, filePath string) mockaur.GetFunc {
	f, err := os.Open(filePath)
	require.NoError(t, err)

	fBytes, err := io.ReadAll(f)
	require.NoError(t, err)

	pkgs := []aur.Pkg{}
	err = json.Unmarshal(fBytes, &pkgs)
	require.NoError(t, err)

	return func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		return pkgs, nil
	}
}

func TestGrapher_GraphFromTargets_jellyfin(t *testing.T) {
	mockDB := &mock.DBExecutor{
		SyncPackageFn: func(string) mock.IPackage { return nil },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "jellyfin":
				return nil
			case "dotnet-runtime-6.0":
				return &mock.Package{
					PName:    "dotnet-runtime-6.0",
					PBase:    "dotnet-runtime-6.0",
					PVersion: "6.0.100-1",
					PDB:      mock.NewDB("community"),
				}
			case "dotnet-sdk-6.0":
				return &mock.Package{
					PName:    "dotnet-sdk-6.0",
					PBase:    "dotnet-sdk-6.0",
					PVersion: "6.0.100-1",
					PDB:      mock.NewDB("community"),
				}
			}

			return nil
		},
		PackagesFromGroupFn: func(string) []mock.IPackage { return nil },
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "dotnet-sdk-6.0", "dotnet-runtime-6.0", "jellyfin-server=10.8.8", "jellyfin-web=10.8.8":
				return false
			}

			return true
		},
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		if query.Needles[0] == "jellyfin" {
			jfinFn := getFromFile(t, "testdata/jellyfin.json")
			return jfinFn(ctx, query)
		}

		if query.Needles[0] == "jellyfin-web" {
			jfinWebFn := getFromFile(t, "testdata/jellyfin-web.json")
			return jfinWebFn(ctx, query)
		}

		if query.Needles[0] == "jellyfin-server" {
			jfinServerFn := getFromFile(t, "testdata/jellyfin-server.json")
			return jfinServerFn(ctx, query)
		}

		panic(fmt.Sprintf("implement me %v", query.Needles))
	}}

	type fields struct {
		dbExecutor  db.Executor
		aurCache    aurc.QueryClient
		noDeps      bool
		noCheckDeps bool
	}
	type args struct {
		targets []string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []map[string]*InstallInfo
		wantErr bool
	}{
		{
			name: "noDeps",
			fields: fields{
				dbExecutor:  mockDB,
				aurCache:    mockAUR,
				noDeps:      true,
				noCheckDeps: false,
			},
			args: args{
				targets: []string{"jellyfin"},
			},
			want: []map[string]*InstallInfo{
				{
					"jellyfin": {
						Source:  AUR,
						Reason:  Explicit,
						Version: "10.8.8-1",
						AURBase: ptrString("jellyfin"),
					},
				},
				{
					"dotnet-sdk-6.0": {
						Source:     Sync,
						Reason:     MakeDep,
						Version:    "6.0.100-1",
						SyncDBName: ptrString("community"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "deps",
			fields: fields{
				dbExecutor:  mockDB,
				aurCache:    mockAUR,
				noDeps:      false,
				noCheckDeps: false,
			},
			args: args{
				targets: []string{"jellyfin"},
			},
			want: []map[string]*InstallInfo{
				{
					"jellyfin": {
						Source:  AUR,
						Reason:  Explicit,
						Version: "10.8.8-1",
						AURBase: ptrString("jellyfin"),
					},
				},
				{
					"jellyfin-web": {
						Source:  AUR,
						Reason:  Dep,
						Version: "10.8.8-1",
						AURBase: ptrString("jellyfin"),
					},
					"jellyfin-server": {
						Source:  AUR,
						Reason:  Dep,
						Version: "10.8.8-1",
						AURBase: ptrString("jellyfin"),
					},
				},
				{
					"dotnet-sdk-6.0": {
						Source:     Sync,
						Reason:     MakeDep,
						Version:    "6.0.100-1",
						SyncDBName: ptrString("community"),
					},
					"dotnet-runtime-6.0": {
						Source:     Sync,
						Reason:     Dep,
						Version:    "6.0.100-1",
						SyncDBName: ptrString("community"),
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGrapher(tt.fields.dbExecutor,
				tt.fields.aurCache, false, true,
				tt.fields.noDeps, tt.fields.noCheckDeps, false,
				text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
			got, err := g.GraphFromTargets(context.Background(), nil, tt.args.targets)
			require.NoError(t, err)
			layers := got.TopoSortedLayerMap(nil)
			require.EqualValues(t, tt.want, layers, layers)
		})
	}
}

func TestGrapher_GraphProvides_androidsdk(t *testing.T) {
	mockDB := &mock.DBExecutor{
		SyncPackageFn: func(string) mock.IPackage { return nil },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "android-sdk":
				return nil
			case "jdk11-openjdk":
				return &mock.Package{
					PName:    "jdk11-openjdk",
					PVersion: "11.0.12.u7-1",
					PDB:      mock.NewDB("community"),
					PProvides: mock.DependList{
						Depends: []alpm.Depend{
							{Name: "java-environment", Version: "11", Mod: alpm.DepModEq},
							{Name: "java-environment-openjdk", Version: "11", Mod: alpm.DepModEq},
							{Name: "jdk11-openjdk", Version: "11.0.19.u7-1", Mod: alpm.DepModEq},
						},
					},
				}
			case "java-environment":
				panic("not supposed to be called")
			}
			panic("implement me " + s)
		},
		PackagesFromGroupFn: func(string) []mock.IPackage { return nil },
		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "java-environment":
				return false
			}

			switch s {
			case "libxtst", "fontconfig", "freetype2", "lib32-gcc-libs", "lib32-glibc", "libx11", "libxext", "libxrender", "zlib", "gcc-libs":
				return true
			}

			panic("implement me " + s)
		},
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		if query.Needles[0] == "android-sdk" {
			jfinFn := getFromFile(t, "testdata/android-sdk.json")
			return jfinFn(ctx, query)
		}

		panic(fmt.Sprintf("implement me %v", query.Needles))
	}}

	type fields struct {
		dbExecutor  db.Executor
		aurCache    aurc.QueryClient
		noDeps      bool
		noCheckDeps bool
	}
	type args struct {
		targets []string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []map[string]*InstallInfo
		wantErr bool
	}{
		{
			name: "explicit dep",
			fields: fields{
				dbExecutor:  mockDB,
				aurCache:    mockAUR,
				noDeps:      false,
				noCheckDeps: false,
			},
			args: args{
				targets: []string{"android-sdk", "jdk11-openjdk"},
			},
			want: []map[string]*InstallInfo{
				{
					"android-sdk": {
						Source:  AUR,
						Reason:  Explicit,
						Version: "26.1.1-2",
						AURBase: ptrString("android-sdk"),
					},
				},
				{
					"jdk11-openjdk": {
						Source:     Sync,
						Reason:     Explicit,
						Version:    "11.0.12.u7-1",
						SyncDBName: ptrString("community"),
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGrapher(tt.fields.dbExecutor,
				tt.fields.aurCache, false, true,
				tt.fields.noDeps, tt.fields.noCheckDeps, false,
				text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
			got, err := g.GraphFromTargets(context.Background(), nil, tt.args.targets)
			require.NoError(t, err)
			layers := got.TopoSortedLayerMap(nil)
			require.EqualValues(t, tt.want, layers, layers)
		})
	}
}
