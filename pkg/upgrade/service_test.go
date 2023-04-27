package upgrade

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Jguer/aur"
	"github.com/Jguer/go-alpm/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/db/mock"
	"github.com/Jguer/yay/v12/pkg/dep"
	"github.com/Jguer/yay/v12/pkg/query"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/topo"
	"github.com/Jguer/yay/v12/pkg/vcs"

	mockaur "github.com/Jguer/yay/v12/pkg/dep/mock"
)

func ptrString(s string) *string {
	return &s
}

func TestUpgradeService_GraphUpgrades(t *testing.T) {
	t.Parallel()
	linuxDepInfo := &dep.InstallInfo{
		Reason:       dep.Explicit,
		Source:       dep.Sync,
		AURBase:      nil,
		LocalVersion: "4.5.0-1",
		Version:      "5.0.0-1",
		SyncDBName:   ptrString("core"),
		Upgrade:      true,
		Devel:        false,
	}

	exampleDepInfoDevel := &dep.InstallInfo{
		Source:       dep.AUR,
		Reason:       dep.Dep,
		AURBase:      ptrString("example"),
		LocalVersion: "2.2.1.r32.41baa362-1",
		Version:      "latest-commit",
		Upgrade:      true,
		Devel:        true,
	}

	newDepInfo := &dep.InstallInfo{
		Source:       dep.Sync,
		Reason:       dep.Dep,
		SyncDBName:   ptrString("core"),
		Version:      "3.0.1-2",
		LocalVersion: "",
		Upgrade:      true,
		Devel:        false,
	}

	exampleDepInfoAUR := &dep.InstallInfo{
		Source:       dep.AUR,
		Reason:       dep.Dep,
		AURBase:      ptrString("example"),
		LocalVersion: "2.2.1.r32.41baa362-1",
		Version:      "2.2.1.r69.g8a10460-1",
		Upgrade:      true,
		Devel:        false,
	}

	yayDepInfo := &dep.InstallInfo{
		Reason:       dep.Explicit,
		Source:       dep.AUR,
		AURBase:      ptrString("yay"),
		LocalVersion: "10.2.3",
		Version:      "10.2.4",
		Upgrade:      true,
		Devel:        false,
	}

	coreDB := mock.NewDB("core")
	dbExe := &mock.DBExecutor{
		InstalledRemotePackageNamesFn: func() []string {
			return []string{"yay", "example-git"}
		},
		InstalledRemotePackagesFn: func() map[string]mock.IPackage {
			mapRemote := make(map[string]mock.IPackage)
			mapRemote["yay"] = &mock.Package{
				PName:    "yay",
				PBase:    "yay",
				PVersion: "10.2.3",
				PReason:  alpm.PkgReasonExplicit,
			}

			mapRemote["example-git"] = &mock.Package{
				PName:    "example-git",
				PBase:    "example",
				PVersion: "2.2.1.r32.41baa362-1",
				PReason:  alpm.PkgReasonDepend,
			}

			return mapRemote
		},
		LocalSatisfierExistsFn: func(string) bool { return false },
		SyncSatisfierFn: func(s string) mock.IPackage {
			return &mock.Package{
				PName:    "new-dep",
				PVersion: "3.0.1-2",
				PDB:      coreDB,
			}
		},
		SyncUpgradesFn: func(bool) (map[string]db.SyncUpgrade, error) {
			mapUpgrades := make(map[string]db.SyncUpgrade)

			mapUpgrades["linux"] = db.SyncUpgrade{
				Package: &mock.Package{
					PName:    "linux",
					PVersion: "5.0.0-1",
					PReason:  alpm.PkgReasonDepend,
					PDB:      coreDB,
					PDepends: mock.DependList{Depends: []alpm.Depend{
						{Name: "new-dep", Version: "3.0.1"},
					}},
				},
				LocalVersion: "4.5.0-1",
				Reason:       alpm.PkgReasonExplicit,
			}

			mapUpgrades["new-dep"] = db.SyncUpgrade{
				Package: &mock.Package{
					PName:    "new-dep",
					PVersion: "3.0.1-2",
					PReason:  alpm.PkgReasonDepend,
					PDB:      coreDB,
				},
				LocalVersion: "",
				Reason:       alpm.PkgReasonDepend,
			}

			return mapUpgrades, nil
		},
		ReposFn: func() []string { return []string{"core"} },
	}
	vcsStore := &vcs.Mock{
		ToUpgradeReturn: []string{"example-git"},
	}

	mockAUR := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{
				{Name: "yay", Version: "10.2.4", PackageBase: "yay"},
				{
					Name: "example-git", Version: "2.2.1.r69.g8a10460-1",
					PackageBase: "example", Depends: []string{"new-dep"},
				},
			}, nil
		},
	}
	type fields struct {
		input     io.Reader
		output    io.Writer
		noConfirm bool
		devel     bool
	}
	type args struct {
		graph           *topo.Graph[string, *dep.InstallInfo]
		enableDowngrade bool
	}
	tests := []struct {
		name         string
		fields       fields
		args         args
		mustExist    map[string]*dep.InstallInfo
		mustNotExist map[string]bool
		wantExclude  []string
		wantErr      bool
	}{
		{
			name: "no input",
			fields: fields{
				input:     strings.NewReader("\n"),
				output:    io.Discard,
				noConfirm: false,
			},
			args: args{
				graph:           nil,
				enableDowngrade: false,
			},
			mustExist: map[string]*dep.InstallInfo{
				"yay":         yayDepInfo,
				"linux":       linuxDepInfo,
				"example-git": exampleDepInfoAUR,
				"new-dep":     newDepInfo,
			},
			mustNotExist: map[string]bool{},
			wantErr:      false,
			wantExclude:  []string{},
		},
		{
			name: "no input devel",
			fields: fields{
				input:     strings.NewReader("\n"),
				output:    io.Discard,
				noConfirm: false,
				devel:     true,
			},
			args: args{
				graph:           nil,
				enableDowngrade: false,
			},
			mustExist: map[string]*dep.InstallInfo{
				"yay":         yayDepInfo,
				"linux":       linuxDepInfo,
				"example-git": exampleDepInfoDevel,
			},
			mustNotExist: map[string]bool{},
			wantErr:      false,
			wantExclude:  []string{},
		},
		{
			name: "exclude example-git",
			fields: fields{
				input:     strings.NewReader("2\n"),
				output:    io.Discard,
				noConfirm: false,
			},
			args: args{
				graph:           nil,
				enableDowngrade: false,
			},
			mustExist: map[string]*dep.InstallInfo{
				"yay":   yayDepInfo,
				"linux": linuxDepInfo,
			},
			mustNotExist: map[string]bool{"example-git": true, "new-dep": true},
			wantErr:      false,
			wantExclude:  []string{"example-git", "new-dep"},
		},
		{
			name: "exclude new-dep should have no effect",
			fields: fields{
				input:     strings.NewReader("1 3 4\n"),
				output:    io.Discard,
				noConfirm: false,
			},
			args: args{
				graph:           nil,
				enableDowngrade: false,
			},
			mustExist: map[string]*dep.InstallInfo{
				"example-git": exampleDepInfoAUR,
				"new-dep":     newDepInfo,
			},
			mustNotExist: map[string]bool{"linux": true, "yay": true},
			wantErr:      false,
			wantExclude:  []string{"linux", "yay"},
		},
		{
			name: "exclude yay",
			fields: fields{
				input:     strings.NewReader("1\n"),
				output:    io.Discard,
				noConfirm: false,
			},
			args: args{
				graph:           nil,
				enableDowngrade: false,
			},
			mustExist: map[string]*dep.InstallInfo{
				"linux":       linuxDepInfo,
				"example-git": exampleDepInfoAUR,
			},
			mustNotExist: map[string]bool{"yay": true},
			wantErr:      false,
			wantExclude:  []string{"yay"},
		},
		{
			name: "exclude linux",
			fields: fields{
				input:     strings.NewReader("3\n"),
				output:    io.Discard,
				noConfirm: false,
			},
			args: args{
				graph:           nil,
				enableDowngrade: false,
			},
			mustExist: map[string]*dep.InstallInfo{
				"yay":         yayDepInfo,
				"example-git": exampleDepInfoAUR,
				"new-dep":     newDepInfo,
			},
			mustNotExist: map[string]bool{"linux": true},
			wantErr:      false,
			wantExclude:  []string{"linux"},
		},
		{
			name: "only linux",
			fields: fields{
				input:     strings.NewReader("^3\n"),
				output:    io.Discard,
				noConfirm: false,
			},
			args: args{
				graph:           nil,
				enableDowngrade: false,
			},
			mustExist: map[string]*dep.InstallInfo{
				"linux": linuxDepInfo,
			},
			mustNotExist: map[string]bool{"yay": true, "example-git": true},
			wantErr:      false,
			wantExclude:  []string{"yay", "example-git", "new-dep"},
		},
		{
			name: "exclude all",
			fields: fields{
				input:     strings.NewReader("1-4\n"),
				output:    io.Discard,
				noConfirm: false,
			},
			args: args{
				graph:           nil,
				enableDowngrade: false,
			},
			mustExist:    map[string]*dep.InstallInfo{},
			mustNotExist: map[string]bool{"yay": true, "example-git": true, "linux": true},
			wantErr:      false,
			wantExclude:  []string{"yay", "example-git", "linux", "new-dep"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grapher := dep.NewGrapher(dbExe, mockAUR,
				false, true, false, false, false, text.NewLogger(tt.fields.output, os.Stderr,
					tt.fields.input, true, "test"))

			cfg := &settings.Configuration{
				Devel: tt.fields.devel, Mode: parser.ModeAny,
			}

			logger := text.NewLogger(tt.fields.output, os.Stderr,
				tt.fields.input, true, "test")
			u := &UpgradeService{
				log:         logger,
				grapher:     grapher,
				aurCache:    mockAUR,
				dbExecutor:  dbExe,
				vcsStore:    vcsStore,
				cfg:         cfg,
				noConfirm:   tt.fields.noConfirm,
				AURWarnings: query.NewWarnings(logger),
			}

			got, err := u.GraphUpgrades(context.Background(), tt.args.graph, tt.args.enableDowngrade, func(*Upgrade) bool { return true })
			if (err != nil) != tt.wantErr {
				t.Errorf("UpgradeService.GraphUpgrades() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			excluded, err := u.UserExcludeUpgrades(got)
			require.NoError(t, err)

			for node, info := range tt.mustExist {
				assert.True(t, got.Exists(node), node)
				assert.Equal(t, info, got.GetNodeInfo(node).Value)
			}

			for node := range tt.mustNotExist {
				assert.False(t, got.Exists(node), node)
			}

			assert.ElementsMatch(t, tt.wantExclude, excluded)
		})
	}
}

func TestUpgradeService_GraphUpgradesNoUpdates(t *testing.T) {
	t.Parallel()
	dbExe := &mock.DBExecutor{
		InstalledRemotePackageNamesFn: func() []string {
			return []string{"yay", "example-git"}
		},
		InstalledRemotePackagesFn: func() map[string]mock.IPackage {
			mapRemote := make(map[string]mock.IPackage)
			mapRemote["yay"] = &mock.Package{
				PName:    "yay",
				PBase:    "yay",
				PVersion: "10.2.3",
				PReason:  alpm.PkgReasonExplicit,
			}

			mapRemote["example-git"] = &mock.Package{
				PName:    "example-git",
				PBase:    "example",
				PVersion: "2.2.1.r32.41baa362-1",
				PReason:  alpm.PkgReasonDepend,
			}

			return mapRemote
		},
		SyncUpgradesFn: func(bool) (map[string]db.SyncUpgrade, error) {
			mapUpgrades := make(map[string]db.SyncUpgrade)
			return mapUpgrades, nil
		},
		ReposFn: func() []string { return []string{"core"} },
	}
	vcsStore := &vcs.Mock{
		ToUpgradeReturn: []string{},
	}

	mockAUR := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{}, nil
		},
	}
	type fields struct {
		input     io.Reader
		output    io.Writer
		noConfirm bool
		devel     bool
	}
	type args struct {
		graph           *topo.Graph[string, *dep.InstallInfo]
		enableDowngrade bool
	}
	tests := []struct {
		name         string
		fields       fields
		args         args
		mustExist    map[string]*dep.InstallInfo
		mustNotExist map[string]bool
		wantExclude  []string
		wantErr      bool
	}{
		{
			name: "no input",
			fields: fields{
				input:     strings.NewReader(""),
				output:    io.Discard,
				noConfirm: false,
			},
			args: args{
				graph:           nil,
				enableDowngrade: false,
			},
			mustExist:    map[string]*dep.InstallInfo{},
			mustNotExist: map[string]bool{},
			wantErr:      false,
			wantExclude:  []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grapher := dep.NewGrapher(dbExe, mockAUR,
				false, true, false, false, false, text.NewLogger(tt.fields.output, os.Stderr,
					tt.fields.input, true, "test"))

			cfg := &settings.Configuration{
				Devel: tt.fields.devel,
				Mode:  parser.ModeAny,
			}

			logger := text.NewLogger(tt.fields.output, os.Stderr,
				tt.fields.input, true, "test")
			u := &UpgradeService{
				log:         logger,
				grapher:     grapher,
				aurCache:    mockAUR,
				dbExecutor:  dbExe,
				vcsStore:    vcsStore,
				cfg:         cfg,
				noConfirm:   tt.fields.noConfirm,
				AURWarnings: query.NewWarnings(logger),
			}

			got, err := u.GraphUpgrades(context.Background(), tt.args.graph, tt.args.enableDowngrade, func(*Upgrade) bool { return true })
			if (err != nil) != tt.wantErr {
				t.Errorf("UpgradeService.GraphUpgrades() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			excluded, err := u.UserExcludeUpgrades(got)
			require.NoError(t, err)

			for node, info := range tt.mustExist {
				assert.True(t, got.Exists(node), node)
				assert.Equal(t, info, got.GetNodeInfo(node).Value)
			}

			for node := range tt.mustNotExist {
				assert.False(t, got.Exists(node), node)
			}

			assert.ElementsMatch(t, tt.wantExclude, excluded)
		})
	}
}

func TestUpgradeService_Warnings(t *testing.T) {
	t.Parallel()
	dbExe := &mock.DBExecutor{
		InstalledRemotePackageNamesFn: func() []string {
			return []string{"orphan", "outdated", "missing", "orphan-ignored"}
		},
		InstalledRemotePackagesFn: func() map[string]mock.IPackage {
			mapRemote := make(map[string]mock.IPackage)
			mapRemote["orphan"] = &mock.Package{
				PName:    "orphan",
				PBase:    "orphan",
				PVersion: "10.2.3",
				PReason:  alpm.PkgReasonExplicit,
			}

			mapRemote["outdated"] = &mock.Package{
				PName:    "outdated",
				PBase:    "outdated",
				PVersion: "10.2.3",
				PReason:  alpm.PkgReasonExplicit,
			}

			mapRemote["missing"] = &mock.Package{
				PName:    "missing",
				PBase:    "missing",
				PVersion: "10.2.3",
				PReason:  alpm.PkgReasonExplicit,
			}

			mapRemote["orphan-ignored"] = &mock.Package{
				PName:         "orphan-ignored",
				PBase:         "orphan-ignored",
				PVersion:      "10.2.3",
				PReason:       alpm.PkgReasonExplicit,
				PShouldIgnore: true,
			}

			return mapRemote
		},
		LocalSatisfierExistsFn: func(string) bool { return false },
		SyncSatisfierFn: func(s string) mock.IPackage {
			return nil
		},
		SyncUpgradesFn: func(bool) (map[string]db.SyncUpgrade, error) {
			mapUpgrades := make(map[string]db.SyncUpgrade)
			return mapUpgrades, nil
		},
		ReposFn: func() []string { return []string{"core"} },
	}
	vcsStore := &vcs.Mock{
		ToUpgradeReturn: []string{},
	}

	mockAUR := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{
				{
					Name: "outdated", Version: "10.2.4", PackageBase: "orphan",
					OutOfDate: 100, Maintainer: "bob",
				},
				{
					Name: "orphan", Version: "10.2.4", PackageBase: "orphan",
					Maintainer: "",
				},
			}, nil
		},
	}

	logger := text.NewLogger(io.Discard, os.Stderr,
		strings.NewReader("\n"), true, "test")
	grapher := dep.NewGrapher(dbExe, mockAUR,
		false, true, false, false, false, logger)

	cfg := &settings.Configuration{
		Devel: false, Mode: parser.ModeAUR,
	}

	u := &UpgradeService{
		log:         logger,
		grapher:     grapher,
		aurCache:    mockAUR,
		dbExecutor:  dbExe,
		vcsStore:    vcsStore,
		cfg:         cfg,
		noConfirm:   true,
		AURWarnings: query.NewWarnings(logger),
	}

	_, err := u.GraphUpgrades(context.Background(), nil, false, func(*Upgrade) bool { return true })
	require.NoError(t, err)

	assert.Equal(t, []string{"missing"}, u.AURWarnings.Missing)
	assert.Equal(t, []string{"outdated"}, u.AURWarnings.OutOfDate)
	assert.Equal(t, []string{"orphan"}, u.AURWarnings.Orphans)
}

func TestUpgradeService_GraphUpgrades_zfs_dkms(t *testing.T) {
	t.Parallel()
	zfsDKMSInfo := &dep.InstallInfo{
		Reason:       dep.Explicit,
		Source:       dep.AUR,
		AURBase:      ptrString("zfs-dkms"),
		LocalVersion: "2.1.10-1",
		Version:      "2.1.11-1",
		Upgrade:      true,
		Devel:        false,
	}

	zfsUtilsInfo := &dep.InstallInfo{
		Reason:       dep.Dep,
		Source:       dep.AUR,
		AURBase:      ptrString("zfs-utils"),
		LocalVersion: "2.1.10-1",
		Version:      "2.1.11-1",
		Upgrade:      true,
		Devel:        false,
	}

	vcsStore := &vcs.Mock{ToUpgradeReturn: []string{}}

	mockAUR := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			if len(query.Needles) == 2 {
				return []aur.Pkg{
					{
						Name: "zfs-dkms", Version: "2.1.11-1",
						PackageBase: "zfs-dkms", Depends: []string{"zfs-utils=2.1.11"},
					},
					{Name: "zfs-utils", Version: "2.1.11-1", PackageBase: "zfs-utils"},
				}, nil
			}
			if len(query.Needles) == 1 {
				return []aur.Pkg{
					{Name: "zfs-utils", Version: "2.1.11-1", PackageBase: "zfs-utils"},
				}, nil
			}
			panic("not implemented")
		},
	}
	type fields struct {
		input     io.Reader
		noConfirm bool
		devel     bool
	}
	type args struct {
		graph           *topo.Graph[string, *dep.InstallInfo]
		enableDowngrade bool
	}
	tests := []struct {
		name           string
		fields         fields
		args           args
		mustExist      map[string]*dep.InstallInfo
		mustNotExist   map[string]bool
		wantExclude    []string
		wantErr        bool
		remotePackages []string
	}{
		{
			name: "no input",
			fields: fields{
				input:     strings.NewReader("\n"),
				noConfirm: false,
			},
			args: args{
				graph:           nil,
				enableDowngrade: false,
			},
			mustExist: map[string]*dep.InstallInfo{
				"zfs-dkms":  zfsDKMSInfo,
				"zfs-utils": zfsUtilsInfo,
			},
			remotePackages: []string{"zfs-utils", "zfs-dkms"},
			mustNotExist:   map[string]bool{},
			wantErr:        false,
			wantExclude:    []string{},
		},
		{
			name: "no input - inverted order",
			fields: fields{
				input:     strings.NewReader("\n"),
				noConfirm: false,
			},
			args: args{
				graph:           nil,
				enableDowngrade: false,
			},
			mustExist: map[string]*dep.InstallInfo{
				"zfs-dkms":  zfsDKMSInfo,
				"zfs-utils": zfsUtilsInfo,
			},
			remotePackages: []string{"zfs-dkms", "zfs-utils"},
			mustNotExist:   map[string]bool{},
			wantErr:        false,
			wantExclude:    []string{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dbExe := &mock.DBExecutor{
				InstalledRemotePackageNamesFn: func() []string {
					return tt.remotePackages
				},
				InstalledRemotePackagesFn: func() map[string]mock.IPackage {
					mapRemote := make(map[string]mock.IPackage)
					mapRemote["zfs-dkms"] = &mock.Package{
						PName:    "zfs-dkms",
						PBase:    "zfs-dkms",
						PVersion: "2.1.10-1",
						PReason:  alpm.PkgReasonExplicit,
						PDepends: mock.DependList{Depends: []alpm.Depend{
							{Name: "zfs-utils", Version: "2.1.10-1"},
						}},
					}

					mapRemote["zfs-utils"] = &mock.Package{
						PName:    "zfs-utils",
						PBase:    "zfs-utils",
						PVersion: "2.1.10-1",
						PReason:  alpm.PkgReasonDepend,
					}

					return mapRemote
				},
				LocalSatisfierExistsFn: func(string) bool { return false },
				SyncSatisfierFn: func(s string) mock.IPackage {
					return nil
				},
				SyncUpgradesFn: func(bool) (map[string]db.SyncUpgrade, error) {
					mapUpgrades := make(map[string]db.SyncUpgrade)
					return mapUpgrades, nil
				},
				ReposFn: func() []string { return []string{"core"} },
			}

			logger := text.NewLogger(io.Discard, os.Stderr,
				tt.fields.input, true, "test")
			grapher := dep.NewGrapher(dbExe, mockAUR,
				false, true, false, false, false, logger)

			cfg := &settings.Configuration{
				Devel: tt.fields.devel, Mode: parser.ModeAny,
			}

			u := &UpgradeService{
				log:         logger,
				grapher:     grapher,
				aurCache:    mockAUR,
				dbExecutor:  dbExe,
				vcsStore:    vcsStore,
				cfg:         cfg,
				noConfirm:   tt.fields.noConfirm,
				AURWarnings: query.NewWarnings(logger),
			}

			got, err := u.GraphUpgrades(context.Background(), tt.args.graph, tt.args.enableDowngrade, func(*Upgrade) bool { return true })
			if (err != nil) != tt.wantErr {
				t.Errorf("UpgradeService.GraphUpgrades() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			excluded, err := u.UserExcludeUpgrades(got)
			require.NoError(t, err)

			for node, info := range tt.mustExist {
				assert.True(t, got.Exists(node), node)
				assert.Equal(t, info, got.GetNodeInfo(node).Value)
			}

			for node := range tt.mustNotExist {
				assert.False(t, got.Exists(node), node)
			}

			assert.ElementsMatch(t, tt.wantExclude, excluded)
		})
	}
}
