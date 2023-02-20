package dep

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"

	aurc "github.com/Jguer/aur"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/db/mock"
	mockaur "github.com/Jguer/yay/v11/pkg/dep/mock"
	aur "github.com/Jguer/yay/v11/pkg/query"
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

		panic("implement me")
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
				tt.fields.aurCache, false, true, os.Stdout,
				tt.fields.noDeps, tt.fields.noCheckDeps)
			got, err := g.GraphFromTargets(context.Background(), nil, tt.args.targets)
			require.NoError(t, err)
			layers := got.TopoSortedLayerMap(nil)
			require.EqualValues(t, tt.want, layers, layers)
		})
	}
}
