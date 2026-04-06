//go:build !integration
// +build !integration

package dep

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	aurc "github.com/Jguer/aur"
	alpm "github.com/Jguer/dyalpm"
	mapset "github.com/deckarep/golang-set/v2"
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

func getFromFile(t testing.TB, filePath string) mockaur.GetFunc {
	t.Helper()
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

func TestGrapher_findDepsFromAUR_logsRequiredByForMissingDep(t *testing.T) {
	t.Parallel()

	mockDB := &mock.DBExecutor{}
	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		return []aur.Pkg{}, nil
	}}

	var stderr bytes.Buffer
	logger := text.NewLogger(io.Discard, &stderr, strings.NewReader(""), true, "test")

	g := NewGrapher(mockDB, mockAUR, false, true, false, false, false, logger)

	graph := NewGraph()

	depString := "missingdep>=1.0"
	depName := "missingdep"
	require.NoError(t, graph.DependOn("existingNeeds", depName))

	toFind := mapset.NewThreadUnsafeSet(depString)
	_ = g.findDepsFromAUR(context.Background(), graph, "currentNeeds", toFind)

	out := stderr.String()
	require.Contains(t, out, "No AUR package found for "+depString+" (required by:")
	require.Contains(t, out, "currentNeeds")
	require.Contains(t, out, "existingNeeds")
}

func TestGrapher_GraphFromTargets_jellyfin(t *testing.T) {
	t.Parallel()

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
		LocalPackageFn: func(string) mock.IPackage { return nil },
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
			layers := got.TopoSortedLayers(nil)
			require.EqualValues(t, tt.want, layers, layers)
		})
	}
}

func TestGrapher_GraphProvides_androidsdk(t *testing.T) {
	t.Parallel()

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
							{Name: "java-environment", Version: "11", Mod: alpm.DepModEQ},
							{Name: "java-environment-openjdk", Version: "11", Mod: alpm.DepModEQ},
							{Name: "jdk11-openjdk", Version: "11.0.19.u7-1", Mod: alpm.DepModEQ},
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
		LocalPackageFn: func(string) mock.IPackage { return nil },
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
			layers := got.TopoSortedLayers(nil)
			require.EqualValues(t, tt.want, layers, layers)
		})
	}
}

func TestGrapher_GraphFromAUR_Deps_ceph_bin(t *testing.T) {
	t.Parallel()

	mockDB := &mock.DBExecutor{
		SyncPackageFn:       func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return []mock.IPackage{} },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "ceph-bin", "ceph-libs-bin":
				return nil
			case "ceph", "ceph-libs", "ceph-libs=17.2.6-2":
				return nil
			}

			panic("implement me " + s)
		},

		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "ceph-libs", "ceph-libs=17.2.6-2":
				return false
			case "dep1", "dep2", "dep3", "makedep1", "makedep2", "checkdep1":
				return true
			}

			panic("implement me " + s)
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		mockPkgs := map[string]aur.Pkg{
			"ceph-bin": {
				Name:        "ceph-bin",
				PackageBase: "ceph-bin",
				Version:     "17.2.6-2",
				Depends:     []string{"ceph-libs=17.2.6-2", "dep1"},
				Provides:    []string{"ceph=17.2.6-2"},
			},
			"ceph-libs-bin": {
				Name:        "ceph-libs-bin",
				PackageBase: "ceph-bin",
				Version:     "17.2.6-2",
				Depends:     []string{"dep1", "dep2"},
				Provides:    []string{"ceph-libs=17.2.6-2"},
			},
			"ceph": {
				Name:         "ceph",
				PackageBase:  "ceph",
				Version:      "17.2.6-2",
				Depends:      []string{"ceph-libs=17.2.6-2", "dep1"},
				MakeDepends:  []string{"makedep1"},
				CheckDepends: []string{"checkdep1"},
				Provides:     []string{"ceph=17.2.6-2"},
			},
			"ceph-libs": {
				Name:         "ceph-libs",
				PackageBase:  "ceph",
				Version:      "17.2.6-2",
				Depends:      []string{"dep1", "dep2", "dep3"},
				MakeDepends:  []string{"makedep1", "makedep2"},
				CheckDepends: []string{"checkdep1"},
				Provides:     []string{"ceph-libs=17.2.6-2"},
			},
		}

		pkgs := []aur.Pkg{}
		for _, needle := range query.Needles {
			if pkg, ok := mockPkgs[needle]; ok {
				pkgs = append(pkgs, pkg)
			} else {
				panic(fmt.Sprintf("implement me %v", needle))
			}
		}

		return pkgs, nil
	}}

	installInfos := map[string]*InstallInfo{
		"ceph-bin exp": {
			Source:  AUR,
			Reason:  Explicit,
			Version: "17.2.6-2",
			AURBase: ptrString("ceph-bin"),
		},
		"ceph-libs-bin exp": {
			Source:  AUR,
			Reason:  Explicit,
			Version: "17.2.6-2",
			AURBase: ptrString("ceph-bin"),
		},
		"ceph exp": {
			Source:  AUR,
			Reason:  Explicit,
			Version: "17.2.6-2",
			AURBase: ptrString("ceph"),
		},
		"ceph-libs exp": {
			Source:  AUR,
			Reason:  Explicit,
			Version: "17.2.6-2",
			AURBase: ptrString("ceph"),
		},
		"ceph-libs dep": {
			Source:  AUR,
			Reason:  Dep,
			Version: "17.2.6-2",
			AURBase: ptrString("ceph"),
		},
	}

	tests := []struct {
		name       string
		targets    []string
		wantLayers []map[string]*InstallInfo
		wantErr    bool
	}{
		{
			name:    "ceph-bin ceph-libs-bin",
			targets: []string{"ceph-bin", "ceph-libs-bin"},
			wantLayers: []map[string]*InstallInfo{
				{"ceph-bin": installInfos["ceph-bin exp"]},
				{"ceph-libs-bin": installInfos["ceph-libs-bin exp"]},
			},
			wantErr: false,
		},
		{
			name:    "ceph-libs-bin ceph-bin (reversed order)",
			targets: []string{"ceph-libs-bin", "ceph-bin"},
			wantLayers: []map[string]*InstallInfo{
				{"ceph-bin": installInfos["ceph-bin exp"]},
				{"ceph-libs-bin": installInfos["ceph-libs-bin exp"]},
			},
			wantErr: false,
		},
		{
			name:    "ceph",
			targets: []string{"ceph"},
			wantLayers: []map[string]*InstallInfo{
				{"ceph": installInfos["ceph exp"]},
				{"ceph-libs": installInfos["ceph-libs dep"]},
			},
			wantErr: false,
		},
		{
			name:    "ceph-bin",
			targets: []string{"ceph-bin"},
			wantLayers: []map[string]*InstallInfo{
				{"ceph-bin": installInfos["ceph-bin exp"]},
				{"ceph-libs": installInfos["ceph-libs dep"]},
			},
			wantErr: false,
		},
		{
			name:    "ceph-bin ceph-libs",
			targets: []string{"ceph-bin", "ceph-libs"},
			wantLayers: []map[string]*InstallInfo{
				{"ceph-bin": installInfos["ceph-bin exp"]},
				{"ceph-libs": installInfos["ceph-libs exp"]},
			},
			wantErr: false,
		},
		{
			name:    "ceph-libs ceph-bin (reversed order)",
			targets: []string{"ceph-libs", "ceph-bin"},
			wantLayers: []map[string]*InstallInfo{
				{"ceph-bin": installInfos["ceph-bin exp"]},
				{"ceph-libs": installInfos["ceph-libs exp"]},
			},
			wantErr: false,
		},
		{
			name:    "ceph ceph-libs-bin",
			targets: []string{"ceph", "ceph-libs-bin"},
			wantLayers: []map[string]*InstallInfo{
				{"ceph": installInfos["ceph exp"]},
				{"ceph-libs-bin": installInfos["ceph-libs-bin exp"]},
			},
			wantErr: false,
		},
		{
			name:    "ceph-libs-bin ceph (reversed order)",
			targets: []string{"ceph-libs-bin", "ceph"},
			wantLayers: []map[string]*InstallInfo{
				{"ceph": installInfos["ceph exp"]},
				{"ceph-libs-bin": installInfos["ceph-libs-bin exp"]},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGrapher(mockDB, mockAUR,
				false, true, false, false, false,
				text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
			got, err := g.GraphFromTargets(context.Background(), nil, tt.targets)
			require.NoError(t, err)
			layers := got.TopoSortedLayers(nil)
			require.EqualValues(t, tt.wantLayers, layers, layers)
		})
	}
}

func TestGrapher_GraphFromAUR_Deps_gourou(t *testing.T) {
	t.Parallel()

	mockDB := &mock.DBExecutor{
		SyncPackageFn:       func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return []mock.IPackage{} },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "gourou", "libzip-git":
				return nil
			case "libzip":
				return &mock.Package{
					PName:    "libzip",
					PVersion: "1.9.2-1",
					PDB:      mock.NewDB("extra"),
				}
			}

			panic("implement me " + s)
		},

		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "gourou", "libzip", "libzip-git":
				return false
			case "dep1", "dep2":
				return true
			}

			panic("implement me " + s)
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		mockPkgs := map[string]aur.Pkg{
			"gourou": {
				Name:        "gourou",
				PackageBase: "gourou",
				Version:     "0.8.1",
				Depends:     []string{"libzip"},
			},
			"libzip-git": {
				Name:        "libzip-git",
				PackageBase: "libzip-git",
				Version:     "1.9.2.r159.gb3ac716c-1",
				Depends:     []string{"dep1", "dep2"},
				Provides:    []string{"libzip=1.9.2.r159.gb3ac716c"},
			},
		}

		pkgs := []aur.Pkg{}
		for _, needle := range query.Needles {
			if pkg, ok := mockPkgs[needle]; ok {
				pkgs = append(pkgs, pkg)
			} else {
				panic(fmt.Sprintf("implement me %v", needle))
			}
		}

		return pkgs, nil
	}}

	installInfos := map[string]*InstallInfo{
		"gourou exp": {
			Source:  AUR,
			Reason:  Explicit,
			Version: "0.8.1",
			AURBase: ptrString("gourou"),
		},
		"libzip dep": {
			Source:     Sync,
			Reason:     Dep,
			Version:    "1.9.2-1",
			SyncDBName: ptrString("extra"),
		},
		"libzip exp": {
			Source:     Sync,
			Reason:     Explicit,
			Version:    "1.9.2-1",
			SyncDBName: ptrString("extra"),
		},
		"libzip-git exp": {
			Source:  AUR,
			Reason:  Explicit,
			Version: "1.9.2.r159.gb3ac716c-1",
			AURBase: ptrString("libzip-git"),
		},
	}

	tests := []struct {
		name       string
		targets    []string
		wantLayers []map[string]*InstallInfo
		wantErr    bool
	}{
		{
			name:    "gourou",
			targets: []string{"gourou"},
			wantLayers: []map[string]*InstallInfo{
				{"gourou": installInfos["gourou exp"]},
				{"libzip": installInfos["libzip dep"]},
			},
			wantErr: false,
		},
		{
			name:    "gourou libzip",
			targets: []string{"gourou", "libzip"},
			wantLayers: []map[string]*InstallInfo{
				{"gourou": installInfos["gourou exp"]},
				{"libzip": installInfos["libzip exp"]},
			},
			wantErr: false,
		},
		{
			name:    "gourou libzip-git",
			targets: []string{"gourou", "libzip-git"},
			wantLayers: []map[string]*InstallInfo{
				{"gourou": installInfos["gourou exp"]},
				{"libzip-git": installInfos["libzip-git exp"]},
			},
			wantErr: false,
		},
		{
			name:    "libzip-git gourou (reversed order)",
			targets: []string{"libzip-git", "gourou"},
			wantLayers: []map[string]*InstallInfo{
				{"gourou": installInfos["gourou exp"]},
				{"libzip-git": installInfos["libzip-git exp"]},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGrapher(mockDB, mockAUR,
				false, true, false, false, false,
				text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
			got, err := g.GraphFromTargets(context.Background(), nil, tt.targets)
			require.NoError(t, err)
			layers := got.TopoSortedLayers(nil)
			require.EqualValues(t, tt.wantLayers, layers, layers)
		})
	}
}

func TestGrapher_GraphFromTargets_ReinstalledDeps(t *testing.T) {
	t.Parallel()

	mockDB := &mock.DBExecutor{
		SyncPackageFn:       func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return []mock.IPackage{} },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "gourou":
				return nil
			case "libzip":
				return &mock.Package{
					PName:    "libzip",
					PVersion: "1.9.2-1",
					PDB:      mock.NewDB("extra"),
				}
			}

			panic("implement me " + s)
		},
		SatisfierFromDBFn: func(s, s2 string) (mock.IPackage, error) {
			if s2 == "extra" {
				switch s {
				case "libzip":
					return &mock.Package{
						PName:    "libzip",
						PVersion: "1.9.2-1",
						PDB:      mock.NewDB("extra"),
					}, nil
				}
			}

			panic("implement me " + s2 + "/" + s)
		},

		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "gourou", "libzip":
				return true
			}

			panic("implement me " + s)
		},
		LocalPackageFn: func(s string) mock.IPackage {
			switch s {
			case "libzip":
				return &mock.Package{
					PName:    "libzip",
					PVersion: "1.9.2-1",
					PDB:      mock.NewDB("extra"),
					PReason:  alpm.PkgReasonDepend,
				}
			case "gourou":
				return &mock.Package{
					PName:    "gourou",
					PVersion: "0.8.1",
					PDB:      mock.NewDB("aur"),
					PReason:  alpm.PkgReasonDepend,
				}
			}
			return nil
		},
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		mockPkgs := map[string]aur.Pkg{
			"gourou": {
				Name:        "gourou",
				PackageBase: "gourou",
				Version:     "0.8.1",
				Depends:     []string{"libzip"},
			},
		}

		pkgs := []aur.Pkg{}
		for _, needle := range query.Needles {
			if pkg, ok := mockPkgs[needle]; ok {
				pkgs = append(pkgs, pkg)
			} else {
				panic(fmt.Sprintf("implement me %v", needle))
			}
		}

		return pkgs, nil
	}}

	installInfos := map[string]*InstallInfo{
		"gourou dep": {
			Source:  AUR,
			Reason:  Dep,
			Version: "0.8.1",
			AURBase: ptrString("gourou"),
		},
		"libzip dep": {
			Source:     Sync,
			Reason:     Dep,
			Version:    "1.9.2-1",
			SyncDBName: ptrString("extra"),
		},
	}

	tests := []struct {
		name       string
		targets    []string
		wantLayers []map[string]*InstallInfo
		wantErr    bool
	}{
		{
			name:    "gourou libzip",
			targets: []string{"gourou", "libzip"},
			wantLayers: []map[string]*InstallInfo{
				{"gourou": installInfos["gourou dep"]},
				{"libzip": installInfos["libzip dep"]},
			},
			wantErr: false,
		},
		{
			name:    "aur/gourou extra/libzip",
			targets: []string{"aur/gourou", "extra/libzip"},
			wantLayers: []map[string]*InstallInfo{
				{"gourou": installInfos["gourou dep"]},
				{"libzip": installInfos["libzip dep"]},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGrapher(mockDB, mockAUR,
				false, true, false, false, false,
				text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
			got, err := g.GraphFromTargets(context.Background(), nil, tt.targets)
			require.NoError(t, err)
			layers := got.TopoSortedLayers(nil)
			require.EqualValues(t, tt.wantLayers, layers, layers)
		})
	}
}

func TestGrapher_GraphFromTargets_TargetNotFound(t *testing.T) {
	t.Parallel()

	mockDB := &mock.DBExecutor{
		SyncSatisfierFn:        func(string) mock.IPackage { return nil },
		PackagesFromGroupFn:    func(string) []mock.IPackage { return nil },
		LocalPackageFn:         func(string) mock.IPackage { return nil },
		LocalSatisfierExistsFn: func(string) bool { return false },
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		ok := aur.Pkg{
			Name:        "okpkg",
			PackageBase: "okpkg",
			Version:     "1.0.0",
		}

		switch query.By {
		case aurc.Name:
			// Return only packages that exist.
			pkgs := make([]aur.Pkg, 0, len(query.Needles))
			for _, needle := range query.Needles {
				if needle == ok.Name {
					pkgs = append(pkgs, ok)
				}
			}
			return pkgs, nil
		case aurc.Provides:
			// Provider lookup is done per-target.
			if len(query.Needles) > 0 && query.Needles[0] == ok.Name {
				return []aur.Pkg{ok}, nil
			}
			return []aur.Pkg{}, nil
		default:
			return []aur.Pkg{}, nil
		}
	}}

	g := NewGrapher(mockDB, mockAUR,
		false, true, true, true, false,
		text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))

	t.Run("returns error when all targets are missing", func(t *testing.T) {
		_, err := g.GraphFromTargets(context.Background(), nil, []string{"missing1", "missing2"})
		require.Error(t, err)

		var targetNotFound *aur.ErrTargetNotFound
		require.ErrorAs(t, err, &targetNotFound)
	})

	t.Run("does not error when at least one target is found", func(t *testing.T) {
		got, err := g.GraphFromTargets(context.Background(), nil, []string{"missing1", "okpkg"})
		require.NoError(t, err)

		layers := got.TopoSortedLayers(nil)
		require.EqualValues(t, []map[string]*InstallInfo{
			{
				"okpkg": {
					Source:  AUR,
					Reason:  Explicit,
					Version: "1.0.0",
					AURBase: ptrString("okpkg"),
				},
			},
		}, layers, layers)
	})
}

// TestGrapher_GraphFromAUR_SplitPkgInternalDeps tests split packages where
// packages from the same base depend on each other (like gstreamer-git).
func TestGrapher_GraphFromAUR_SplitPkgInternalDeps(t *testing.T) {
	t.Parallel()

	mockDB := &mock.DBExecutor{
		SyncPackageFn:       func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return []mock.IPackage{} },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			// AUR packages and versioned AUR deps return nil
			case "gstreamer-git", "gst-plugins-base-libs-git", "gst-plugins-good-git",
				"gstreamer-git=1.24.0.r37-1", "gst-plugins-base-libs-git=1.24.0.r37-1":
				return nil
			case "libxml2":
				return &mock.Package{
					PName:    "libxml2",
					PVersion: "2.12.0-1",
					PDB:      mock.NewDB("core"),
				}
			case "glib2":
				return &mock.Package{
					PName:    "glib2",
					PVersion: "2.78.0-1",
					PDB:      mock.NewDB("core"),
				}
			case "orc":
				return &mock.Package{
					PName:    "orc",
					PVersion: "0.4.34-1",
					PDB:      mock.NewDB("extra"),
				}
			case "libxv":
				return &mock.Package{
					PName:    "libxv",
					PVersion: "1.0.12-1",
					PDB:      mock.NewDB("extra"),
				}
			case "iso-codes":
				return &mock.Package{
					PName:    "iso-codes",
					PVersion: "4.15.0-1",
					PDB:      mock.NewDB("extra"),
				}
			case "libpulse":
				return &mock.Package{
					PName:    "libpulse",
					PVersion: "16.1-1",
					PDB:      mock.NewDB("extra"),
				}
			case "wavpack":
				return &mock.Package{
					PName:    "wavpack",
					PVersion: "5.6.0-1",
					PDB:      mock.NewDB("extra"),
				}
			}

			panic("implement me " + s)
		},

		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "gstreamer-git", "gstreamer-git=1.24.0.r37-1",
				"gst-plugins-base-libs-git", "gst-plugins-base-libs-git=1.24.0.r37-1",
				"gst-plugins-good-git":
				return false
			case "libxml2", "glib2", "orc", "libxv", "iso-codes", "libpulse", "wavpack",
				"git", "meson", "ninja": // makedepends
				return true
			}

			panic("implement me " + s)
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		if len(query.Needles) > 0 {
			for _, needle := range query.Needles {
				if needle == "gstreamer-git" || needle == "gst-plugins-base-libs-git" || needle == "gst-plugins-good-git" {
					gstFn := getFromFile(t, "testdata/gstreamer-git.json")
					return gstFn(ctx, query)
				}
			}
		}

		return []aur.Pkg{}, nil // Return empty for unknown packages
	}}

	installInfos := map[string]*InstallInfo{
		"gstreamer-git exp": {
			Source:  AUR,
			Reason:  Explicit,
			Version: "1.24.0.r37-1",
			AURBase: ptrString("gstreamer-git"),
		},
		"gstreamer-git dep": {
			Source:  AUR,
			Reason:  Dep,
			Version: "1.24.0.r37-1",
			AURBase: ptrString("gstreamer-git"),
		},
		"gst-plugins-base-libs-git exp": {
			Source:  AUR,
			Reason:  Explicit,
			Version: "1.24.0.r37-1",
			AURBase: ptrString("gstreamer-git"),
		},
		"gst-plugins-base-libs-git dep": {
			Source:  AUR,
			Reason:  Dep,
			Version: "1.24.0.r37-1",
			AURBase: ptrString("gstreamer-git"),
		},
		"gst-plugins-good-git exp": {
			Source:  AUR,
			Reason:  Explicit,
			Version: "1.24.0.r37-1",
			AURBase: ptrString("gstreamer-git"),
		},
	}

	tests := []struct {
		name       string
		targets    []string
		wantLayers []map[string]*InstallInfo
		wantErr    bool
	}{
		{
			name:    "gst-plugins-good-git pulls in base libs and gstreamer",
			targets: []string{"gst-plugins-good-git"},
			wantLayers: []map[string]*InstallInfo{
				{"gst-plugins-good-git": installInfos["gst-plugins-good-git exp"]},
				{"gst-plugins-base-libs-git": installInfos["gst-plugins-base-libs-git dep"]},
				{"gstreamer-git": installInfos["gstreamer-git dep"]},
			},
			wantErr: false,
		},
		{
			name:    "gst-plugins-base-libs-git pulls in gstreamer",
			targets: []string{"gst-plugins-base-libs-git"},
			wantLayers: []map[string]*InstallInfo{
				{"gst-plugins-base-libs-git": installInfos["gst-plugins-base-libs-git exp"]},
				{"gstreamer-git": installInfos["gstreamer-git dep"]},
			},
			wantErr: false,
		},
		{
			name:    "explicit gstreamer-git with gst-plugins-good-git",
			targets: []string{"gstreamer-git", "gst-plugins-good-git"},
			wantLayers: []map[string]*InstallInfo{
				{"gst-plugins-good-git": installInfos["gst-plugins-good-git exp"]},
				{"gst-plugins-base-libs-git": installInfos["gst-plugins-base-libs-git dep"]},
				{"gstreamer-git": installInfos["gstreamer-git exp"]},
			},
			wantErr: false,
		},
		{
			name:    "all three packages explicitly",
			targets: []string{"gstreamer-git", "gst-plugins-base-libs-git", "gst-plugins-good-git"},
			wantLayers: []map[string]*InstallInfo{
				{"gst-plugins-good-git": installInfos["gst-plugins-good-git exp"]},
				{"gst-plugins-base-libs-git": installInfos["gst-plugins-base-libs-git exp"]},
				{"gstreamer-git": installInfos["gstreamer-git exp"]},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGrapher(mockDB, mockAUR,
				false, true, false, false, false,
				text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
			got, err := g.GraphFromTargets(context.Background(), nil, tt.targets)
			require.NoError(t, err)
			layers := got.TopoSortedLayers(nil)
			require.EqualValues(t, tt.wantLayers, layers, layers)
		})
	}
}

// TestGrapher_GraphFromAUR_CheckDeps tests packages with CheckDepends.
func TestGrapher_GraphFromAUR_CheckDeps(t *testing.T) {
	t.Parallel()

	mockDB := &mock.DBExecutor{
		SyncPackageFn:       func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return []mock.IPackage{} },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "python-pydantic":
				return nil
			case "python":
				return &mock.Package{
					PName:    "python",
					PVersion: "3.11.0-1",
					PDB:      mock.NewDB("core"),
				}
			case "python-typing-extensions":
				return &mock.Package{
					PName:    "python-typing-extensions",
					PVersion: "4.8.0-1",
					PDB:      mock.NewDB("extra"),
				}
			case "python-build":
				return &mock.Package{
					PName:    "python-build",
					PVersion: "1.0.0-1",
					PDB:      mock.NewDB("extra"),
				}
			case "python-installer":
				return &mock.Package{
					PName:    "python-installer",
					PVersion: "0.7.0-1",
					PDB:      mock.NewDB("extra"),
				}
			case "python-pytest":
				return &mock.Package{
					PName:    "python-pytest",
					PVersion: "7.4.0-1",
					PDB:      mock.NewDB("extra"),
				}
			case "python-pytest-mock":
				return &mock.Package{
					PName:    "python-pytest-mock",
					PVersion: "3.11.0-1",
					PDB:      mock.NewDB("extra"),
				}
			}

			panic("implement me " + s)
		},

		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "python-pydantic",
				"python-pytest", "python-pytest-mock": // check deps not installed
				return false
			case "python", "python-typing-extensions", "python-build", "python-installer":
				return true
			}

			panic("implement me " + s)
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		if len(query.Needles) > 0 && query.Needles[0] == "python-pydantic" {
			pydanticFn := getFromFile(t, "testdata/python-pydantic.json")
			return pydanticFn(ctx, query)
		}

		return []aur.Pkg{}, nil // Return empty for unknown packages
	}}

	t.Run("with check deps enabled", func(t *testing.T) {
		g := NewGrapher(mockDB, mockAUR,
			false, true, false, false, false,
			text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
		got, err := g.GraphFromTargets(context.Background(), nil, []string{"python-pydantic"})
		require.NoError(t, err)
		layers := got.TopoSortedLayers(nil)

		// Should have the main package and its check deps
		require.Len(t, layers, 2)
		require.Contains(t, layers[0], "python-pydantic")
		// Check deps should be in the second layer
		require.Contains(t, layers[1], "python-pytest")
		require.Contains(t, layers[1], "python-pytest-mock")
	})

	t.Run("with check deps disabled", func(t *testing.T) {
		g := NewGrapher(mockDB, mockAUR,
			false, true, false, true, false, // noCheckDeps = true
			text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
		got, err := g.GraphFromTargets(context.Background(), nil, []string{"python-pydantic"})
		require.NoError(t, err)
		layers := got.TopoSortedLayers(nil)

		// Should only have the main package (no check deps)
		require.Len(t, layers, 1)
		require.Contains(t, layers[0], "python-pydantic")
	})
}

// TestGrapher_GraphFromAUR_VirtualProvides tests packages that provide virtual packages
// (like mesa-git providing vulkan-driver, opengl-driver).
func TestGrapher_GraphFromAUR_VirtualProvides(t *testing.T) {
	t.Parallel()

	mockDB := &mock.DBExecutor{
		SyncPackageFn:       func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage { return []mock.IPackage{} },
		SyncSatisfierFn: func(s string) mock.IPackage {
			switch s {
			case "mesa-git":
				return nil
			case "libdrm":
				return &mock.Package{
					PName:    "libdrm",
					PVersion: "2.4.117-1",
					PDB:      mock.NewDB("core"),
				}
			case "vulkan-icd-loader":
				return &mock.Package{
					PName:    "vulkan-icd-loader",
					PVersion: "1.3.268-1",
					PDB:      mock.NewDB("extra"),
				}
			case "vulkan-radeon":
				return &mock.Package{
					PName:    "vulkan-radeon",
					PVersion: "23.3.0-1",
					PDB:      mock.NewDB("extra"),
					PProvides: mock.DependList{
						Depends: []alpm.Depend{
							{Name: "vulkan-driver", Version: "", Mod: alpm.DepModAny},
						},
					},
				}
			}

			// Most mesa deps are already installed
			switch s {
			case "libxxf86vm", "libxdamage", "libxshmfence", "libelf", "libunwind",
				"libglvnd", "wayland", "lm_sensors", "zstd", "expat":
				return &mock.Package{
					PName:    s,
					PVersion: "1.0.0-1",
					PDB:      mock.NewDB("extra"),
				}
			}

			panic("implement me " + s)
		},

		LocalSatisfierExistsFn: func(s string) bool {
			switch s {
			case "mesa-git", "vulkan-driver", "opengl-driver":
				return false
			case "libdrm", "libxxf86vm", "libxdamage", "libxshmfence", "libelf",
				"libunwind", "libglvnd", "wayland", "lm_sensors", "vulkan-icd-loader",
				"zstd", "expat",
				"git", "meson", "ninja", "llvm", "clang": // makedepends
				return true
			}

			panic("implement me " + s)
		},
		LocalPackageFn: func(string) mock.IPackage { return nil },
	}

	mockAUR := &mockaur.MockAUR{GetFn: func(ctx context.Context, query *aurc.Query) ([]aur.Pkg, error) {
		if len(query.Needles) > 0 && query.Needles[0] == "mesa-git" {
			mesaFn := getFromFile(t, "testdata/mesa-git.json")
			return mesaFn(ctx, query)
		}

		return []aur.Pkg{}, nil // Return empty for unknown packages
	}}

	t.Run("mesa-git provides vulkan-driver and opengl-driver", func(t *testing.T) {
		g := NewGrapher(mockDB, mockAUR,
			false, true, false, false, false,
			text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
		got, err := g.GraphFromTargets(context.Background(), nil, []string{"mesa-git"})
		require.NoError(t, err)
		layers := got.TopoSortedLayers(nil)

		require.Len(t, layers, 1)
		require.Contains(t, layers[0], "mesa-git")
		require.Equal(t, "24.0.0.r1234-1", layers[0]["mesa-git"].Version)
		require.Equal(t, "mesa-git", *layers[0]["mesa-git"].AURBase)
	})
}
