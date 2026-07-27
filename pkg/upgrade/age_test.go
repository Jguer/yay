//go:build !integration

package upgrade

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/dep"
	"github.com/Jguer/yay/v13/pkg/dep/topo"
	"github.com/Jguer/yay/v13/pkg/settings/exe"
	"github.com/Jguer/yay/v13/pkg/text"
)

func TestApplyMinAge_skipYoung(t *testing.T) {
	t.Parallel()

	const day = 24 * time.Hour
	now := time.Unix(1_700_000_000, 0)
	text.NowFunc = func() time.Time { return now }

	newPkg := now.Add(-2 * day).Unix()

	graph := dep.NewGraph()
	graph.AddNode("new")
	graph.SetNodeInfo("new", &topo.NodeInfo[*dep.InstallInfo]{
		Value: &dep.InstallInfo{Source: dep.AUR, LastModified: newPkg, Upgrade: true},
	})

	report, _ := ApplyMinAge(context.Background(), graph, 7, &MinAgeAdjuster{})
	require.Equal(t, 0, graph.Len())
	require.Len(t, report, 1)
	require.Equal(t, MinAgeSkipped, report[0].Action)
}

func TestApplyMinAge_keepUpgrade(t *testing.T) {
	t.Parallel()

	const day = 24 * time.Hour
	now := time.Unix(1_700_000_000, 0)
	text.NowFunc = func() time.Time { return now }

	newPkg := now.Add(-2 * day).Unix()

	graph := dep.NewGraph()
	graph.AddNode("yay")
	graph.SetNodeInfo("yay", &topo.NodeInfo[*dep.InstallInfo]{
		Value: &dep.InstallInfo{
			Source:       dep.AUR,
			LastModified: newPkg,
			Upgrade:      true,
			LocalVersion: "12.0.0-1",
			Version:      "12.1.0-1",
		},
	})

	report, ignore := ApplyMinAge(context.Background(), graph, 7, &MinAgeAdjuster{})
	require.Equal(t, 0, graph.Len())
	require.Len(t, report, 1)
	require.Equal(t, MinAgeKept, report[0].Action)
	require.Equal(t, "12.0.0-1", report[0].Version)
	require.Equal(t, []string{"yay"}, ignore)
}

func TestApplyMinAge_unknownAgeAllowed(t *testing.T) {
	t.Parallel()

	graph := dep.NewGraph()
	graph.AddNode("sync-pkg")
	graph.SetNodeInfo("sync-pkg", &topo.NodeInfo[*dep.InstallInfo]{
		Value: &dep.InstallInfo{Source: dep.Sync, LastModified: 0, Upgrade: true},
	})

	report, _ := ApplyMinAge(context.Background(), graph, 7, &MinAgeAdjuster{})
	require.Empty(t, report)
	require.Equal(t, 1, graph.Len())
}

func TestApplyMinAge_disabled(t *testing.T) {
	t.Parallel()

	graph := dep.NewGraph()
	graph.AddNode("pkg")
	graph.SetNodeInfo("pkg", &topo.NodeInfo[*dep.InstallInfo]{
		Value: &dep.InstallInfo{Source: dep.AUR, LastModified: time.Now().Unix()},
	})

	report, ignore := ApplyMinAge(context.Background(), graph, 0, &MinAgeAdjuster{})
	require.Nil(t, report)
	require.Nil(t, ignore)
	require.Equal(t, 1, graph.Len())
}

func TestApplyMinAge_downgradeCache(t *testing.T) {
	t.Parallel()

	const day = 24 * time.Hour
	now := time.Unix(1_700_000_000, 0)
	text.NowFunc = func() time.Time { return now }

	newPkg := now.Add(-2 * day).Unix()
	oldBuilt := now.Add(-30 * day).Unix()

	cacheDir := t.TempDir()
	pkgPath := filepath.Join(cacheDir, "foo-1.0.0-1-x86_64.pkg.tar.zst")
	require.NoError(t, os.WriteFile(pkgPath, []byte("x"), 0o600))

	runner := &exe.MockRunner{
		CaptureFn: func(cmd *exec.Cmd) (string, string, error) {
			return fmt.Sprintf("foo|1.0.0-1|%d", oldBuilt), "", nil
		},
	}

	graph := dep.NewGraph()
	info := &dep.InstallInfo{
		Source:       dep.Sync,
		LastModified: newPkg,
		Version:      "2.0.0-1",
		Upgrade:      true,
	}
	graph.AddNode("foo")
	graph.SetNodeInfo("foo", &topo.NodeInfo[*dep.InstallInfo]{Value: info})

	report, ignore := ApplyMinAge(context.Background(), graph, 7, &MinAgeAdjuster{
		CacheDirs:  []string{cacheDir},
		PacmanBin:  "pacman",
		CmdBuilder: &exe.MockBuilder{Runner: runner},
	})
	require.Len(t, report, 1)
	require.Equal(t, MinAgeDowngraded, report[0].Action)
	require.Equal(t, "1.0.0-1", report[0].Version)
	require.Equal(t, pkgPath, info.PkgArchive)
	require.Equal(t, "1.0.0-1", info.Version)
	require.False(t, info.Upgrade)
	require.Equal(t, []string{"foo"}, ignore)
	require.Equal(t, 1, graph.Len())
}

func TestApplyMinAge_downgradeAUR(t *testing.T) {
	t.Parallel()

	const day = 24 * time.Hour
	now := time.Unix(1_700_000_000, 0)
	text.NowFunc = func() time.Time { return now }

	buildDir := t.TempDir()
	pkgDir := filepath.Join(buildDir, "foobase")
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".git"), 0o755))

	newPkg := now.Add(-2 * day).Unix()
	oldTS := now.Add(-30 * day).Unix()
	commit := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	runner := &exe.MockRunner{
		CaptureFn: func(cmd *exec.Cmd) (string, string, error) {
			joined := strings.Join(cmd.Args, " ")
			switch {
			case strings.Contains(joined, " pull "):
				return "", "", nil
			case strings.Contains(joined, " log "):
				return fmt.Sprintf("%d %s", oldTS, commit), "", nil
			case strings.Contains(joined, " show "):
				return testAURSrcinfo, "", nil
			default:
				return "", "", fmt.Errorf("unexpected command: %s", joined)
			}
		},
	}

	graph := dep.NewGraph()
	info := &dep.InstallInfo{
		Source:       dep.AUR,
		AURBase:      "foobase",
		LastModified: newPkg,
		Version:      "9.9.9-1",
		Upgrade:      true,
	}
	graph.AddNode("foobase")
	graph.SetNodeInfo("foobase", &topo.NodeInfo[*dep.InstallInfo]{Value: info})

	report, ignore := ApplyMinAge(context.Background(), graph, 7, &MinAgeAdjuster{
		BuildDir:   buildDir,
		AURURL:     "https://aur.archlinux.org",
		CmdBuilder: &exe.MockBuilder{Runner: runner},
	})
	require.Len(t, report, 1)
	require.Equal(t, MinAgeDowngraded, report[0].Action)
	require.Equal(t, "1.2.3-2", report[0].Version)
	require.Equal(t, commit, info.AURGitRef)
	require.Equal(t, "1.2.3-2", info.Version)
	require.False(t, info.Upgrade)
	require.Empty(t, ignore)
	require.Equal(t, 1, graph.Len())
}

func TestApplyMinAge_skipDependent(t *testing.T) {
	t.Parallel()

	const day = 24 * time.Hour
	now := time.Unix(1_700_000_000, 0)
	text.NowFunc = func() time.Time { return now }

	youngPkg := now.Add(-2 * day).Unix()
	oldPkg := now.Add(-30 * day).Unix()

	graph := dep.NewGraph()
	graph.AddNode("young-lib")
	graph.SetNodeInfo("young-lib", &topo.NodeInfo[*dep.InstallInfo]{
		Value: &dep.InstallInfo{Source: dep.Sync, LastModified: youngPkg, Upgrade: true, Version: "2.0.0-1"},
	})
	graph.AddNode("app")
	graph.SetNodeInfo("app", &topo.NodeInfo[*dep.InstallInfo]{
		Value: &dep.InstallInfo{Source: dep.Sync, LastModified: oldPkg, Upgrade: true, Version: "1.0.0-1"},
	})
	require.NoError(t, graph.DependOn("app", "young-lib"))

	report, _ := ApplyMinAge(context.Background(), graph, 7, &MinAgeAdjuster{})
	require.Equal(t, 0, graph.Len())
	require.Len(t, report, 2)

	byName := make(map[string]MinAgeEntry, len(report))
	for i := range report {
		byName[report[i].Name] = report[i]
	}

	require.Equal(t, MinAgeSkipped, byName["young-lib"].Action)
	require.Equal(t, MinAgeSkipped, byName["app"].Action)
	require.Equal(t, "young-lib", byName["app"].Trigger)
}

func TestApplyMinAge_dryRunSkipsDowngrade(t *testing.T) {
	t.Parallel()

	const day = 24 * time.Hour
	now := time.Unix(1_700_000_000, 0)
	text.NowFunc = func() time.Time { return now }

	newPkg := now.Add(-2 * day).Unix()

	cacheDir := t.TempDir()
	pkgPath := filepath.Join(cacheDir, "foo-1.0.0-1-x86_64.pkg.tar.zst")
	require.NoError(t, os.WriteFile(pkgPath, []byte("x"), 0o600))

	runner := &exe.MockRunner{
		CaptureFn: func(cmd *exec.Cmd) (string, string, error) {
			return "", "", fmt.Errorf("dry run must not query cache: %v", cmd.Args)
		},
	}

	graph := dep.NewGraph()
	graph.AddNode("foo")
	graph.SetNodeInfo("foo", &topo.NodeInfo[*dep.InstallInfo]{
		Value: &dep.InstallInfo{
			Source:       dep.Sync,
			LastModified: newPkg,
			Version:      "2.0.0-1",
			Upgrade:      true,
		},
	})

	report, _ := ApplyMinAge(context.Background(), graph, 7, &MinAgeAdjuster{
		CacheDirs:  []string{cacheDir},
		PacmanBin:  "pacman",
		CmdBuilder: &exe.MockBuilder{Runner: runner},
		DryRun:     true,
	})
	require.Equal(t, 0, graph.Len())
	require.Len(t, report, 1)
	require.Equal(t, MinAgeSkipped, report[0].Action)
}

func TestFindYoungPackage_lexicographic(t *testing.T) {
	t.Parallel()

	const day = 24 * time.Hour
	now := time.Unix(1_700_000_000, 0)
	text.NowFunc = func() time.Time { return now }

	young := now.Add(-2 * day).Unix()

	graph := dep.NewGraph()
	for _, name := range []string{"zzz", "aaa"} {
		graph.AddNode(name)
		graph.SetNodeInfo(name, &topo.NodeInfo[*dep.InstallInfo]{
			Value: &dep.InstallInfo{Source: dep.Sync, LastModified: young, Upgrade: true},
		})
	}

	name, _ := findYoungPackage(graph, 7)
	require.Equal(t, "aaa", name)
}

func TestApplyMinAge_keepUpgradePrunesDependent(t *testing.T) {
	t.Parallel()

	const day = 24 * time.Hour
	now := time.Unix(1_700_000_000, 0)
	text.NowFunc = func() time.Time { return now }

	youngPkg := now.Add(-2 * day).Unix()
	oldPkg := now.Add(-30 * day).Unix()

	graph := dep.NewGraph()
	graph.AddNode("yay")
	graph.SetNodeInfo("yay", &topo.NodeInfo[*dep.InstallInfo]{
		Value: &dep.InstallInfo{
			Source:       dep.AUR,
			LastModified: youngPkg,
			Upgrade:      true,
			LocalVersion: "12.0.0-1",
			Version:      "12.1.0-1",
		},
	})
	graph.AddNode("makedep")
	graph.SetNodeInfo("makedep", &topo.NodeInfo[*dep.InstallInfo]{
		Value: &dep.InstallInfo{Source: dep.Sync, LastModified: oldPkg, Upgrade: true, Version: "1.0.0-1"},
	})
	require.NoError(t, graph.DependOn("yay", "makedep"))

	report, _ := ApplyMinAge(context.Background(), graph, 7, &MinAgeAdjuster{})
	require.Equal(t, 0, graph.Len())
	require.Len(t, report, 2)

	byName := make(map[string]MinAgeEntry, len(report))
	for i := range report {
		byName[report[i].Name] = report[i]
	}

	require.Equal(t, MinAgeKept, byName["yay"].Action)
	require.Equal(t, MinAgeSkipped, byName["makedep"].Action)
	require.Equal(t, "yay", byName["makedep"].Trigger)
}

func TestApplyMinAge_dryRunSkipsAUR(t *testing.T) {
	t.Parallel()

	const day = 24 * time.Hour
	now := time.Unix(1_700_000_000, 0)
	text.NowFunc = func() time.Time { return now }

	buildDir := t.TempDir()
	pkgDir := filepath.Join(buildDir, "foobase")
	require.NoError(t, os.MkdirAll(filepath.Join(pkgDir, ".git"), 0o755))

	newPkg := now.Add(-2 * day).Unix()

	runner := &exe.MockRunner{
		CaptureFn: func(cmd *exec.Cmd) (string, string, error) {
			return "", "", fmt.Errorf("dry run must not query AUR git: %v", cmd.Args)
		},
	}

	graph := dep.NewGraph()
	graph.AddNode("foobase")
	graph.SetNodeInfo("foobase", &topo.NodeInfo[*dep.InstallInfo]{
		Value: &dep.InstallInfo{
			Source:       dep.AUR,
			AURBase:      "foobase",
			LastModified: newPkg,
			Version:      "9.9.9-1",
			Upgrade:      true,
		},
	})

	report, _ := ApplyMinAge(context.Background(), graph, 7, &MinAgeAdjuster{
		BuildDir:   buildDir,
		AURURL:     "https://aur.archlinux.org",
		CmdBuilder: &exe.MockBuilder{Runner: runner},
		DryRun:     true,
	})
	require.Equal(t, 0, graph.Len())
	require.Len(t, report, 1)
	require.Equal(t, MinAgeSkipped, report[0].Action)
}

func TestPrintMinAgeReport(t *testing.T) {
	t.Parallel()

	const day = 24 * time.Hour
	now := time.Unix(1_700_000_000, 0)
	text.NowFunc = func() time.Time { return now }

	var buf strings.Builder
	logger := text.NewLogger(&buf, io.Discard, strings.NewReader(""), false, "test")

	PrintMinAgeReport(logger, []MinAgeEntry{
		{
			Action:        MinAgeKept,
			Name:          "yay",
			Version:       "12.0.0-1",
			RemoteVersion: "12.1.0-1",
			Age:           2 * day,
		},
		{
			Action:        MinAgeDowngraded,
			Name:          "foo",
			Version:       "1.0.0-1",
			RemoteVersion: "2.0.0-1",
			Age:           30 * day,
			SourcePath:    "/var/cache/pacman/pkg/foo.pkg.tar.zst",
		},
		{
			Action:  MinAgeSkipped,
			Name:    "app",
			Trigger: "young-lib",
		},
	}, 7)

	out := buf.String()
	require.Contains(t, out, "yay")
	require.Contains(t, out, "foo")
	require.Contains(t, out, "app")
	require.Contains(t, out, "young-lib")
}
