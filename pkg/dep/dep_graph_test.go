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
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/exe"
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
			g := NewGrapher(tt.fields.dbExecutor, &settings.Configuration{},
				tt.fields.aurCache, &exe.MockBuilder{Runner: &exe.MockRunner{}}, false, true,
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
			g := NewGrapher(tt.fields.dbExecutor, &settings.Configuration{},
				tt.fields.aurCache, &exe.MockBuilder{Runner: &exe.MockRunner{}}, false, true,
				tt.fields.noDeps, tt.fields.noCheckDeps, false,
				text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
			got, err := g.GraphFromTargets(context.Background(), nil, tt.args.targets)
			require.NoError(t, err)
			layers := got.TopoSortedLayerMap(nil)
			require.EqualValues(t, tt.want, layers, layers)
		})
	}
}

func TestGrapher_GraphFromAUR_Deps_ceph_bin(t *testing.T) {
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
			g := NewGrapher(mockDB, &settings.Configuration{}, mockAUR,
				&exe.MockBuilder{Runner: &exe.MockRunner{}}, false, true, false, false, false,
				text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
			got, err := g.GraphFromTargets(context.Background(), nil, tt.targets)
			require.NoError(t, err)
			layers := got.TopoSortedLayerMap(nil)
			require.EqualValues(t, tt.wantLayers, layers, layers)
		})
	}
}

func TestGrapher_GraphFromAUR_Deps_gourou(t *testing.T) {
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
			g := NewGrapher(mockDB, &settings.Configuration{}, mockAUR, &exe.MockBuilder{Runner: &exe.MockRunner{}},
				false, true, false, false, false,
				text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
			got, err := g.GraphFromTargets(context.Background(), nil, tt.targets)
			require.NoError(t, err)
			layers := got.TopoSortedLayerMap(nil)
			require.EqualValues(t, tt.wantLayers, layers, layers)
		})
	}
}

func TestGrapher_GraphFromTargets_ReinstalledDeps(t *testing.T) {
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
			g := NewGrapher(mockDB, &settings.Configuration{}, mockAUR, &exe.MockBuilder{Runner: &exe.MockRunner{}},
				false, true, false, false, false,
				text.NewLogger(io.Discard, io.Discard, &os.File{}, true, "test"))
			got, err := g.GraphFromTargets(context.Background(), nil, tt.targets)
			require.NoError(t, err)
			layers := got.TopoSortedLayerMap(nil)
			require.EqualValues(t, tt.wantLayers, layers, layers)
		})
	}
}
