//go:build !integration
// +build !integration

package runtime_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/db/mock"
	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
)

func TestBuildRuntime(t *testing.T) {
	path := "../../testdata/pacman.conf"

	absPath, err := filepath.Abs(path)
	require.NoError(t, err)

	// Prepare test inputs
	cfg := &settings.Configuration{
		Debug:       true,
		UseRPC:      false,
		AURURL:      "https://aur.archlinux.org",
		AURRPCURL:   "https://aur.archlinux.org/rpc",
		BuildDir:    "/tmp",
		VCSFilePath: "",
		PacmanConf:  absPath,
	}
	cmdArgs := parser.MakeArguments()
	version := "1.0.0"

	// Call the function being tested
	run, err := runtime.NewRuntime(cfg, cmdArgs, version)
	require.NoError(t, err)

	// Assert the function's output
	assert.NotNil(t, run)
	assert.NotNil(t, run.QueryBuilder)
	assert.NotNil(t, run.PacmanConf)
	assert.NotNil(t, run.VCSStore)
	assert.NotNil(t, run.CmdBuilder)
	assert.NotNil(t, run.HTTPClient)
	assert.NotNil(t, run.VoteClient)
	assert.NotNil(t, run.AURClient)
	assert.NotNil(t, run.Logger)
}

func TestBuildRuntimeSearchUsesMetadataCacheWhenRPCDisabled(t *testing.T) {
	path := "../../testdata/pacman.conf"

	absPath, err := filepath.Abs(path)
	require.NoError(t, err)

	buildDir := t.TempDir()
	cacheContent := []byte(`[
		{
			"ID": 1125983,
			"Name": "yay",
			"PackageBaseID": 115973,
			"PackageBase": "yay",
			"Version": "11.3.0-1",
			"Description": "Yet another yogurt. Pacman wrapper and AUR helper written in go.",
			"NumVotes": 1855,
			"Popularity": 39.741927,
			"FirstSubmitted": 1475688004,
			"LastModified": 1660494113
		}
	]`)
	require.NoError(t, os.WriteFile(filepath.Join(buildDir, "aur.json"), cacheContent, 0o600))

	cfg := &settings.Configuration{
		Debug:       true,
		UseRPC:      false,
		AURURL:      "https://aur.archlinux.org",
		AURRPCURL:   "http://127.0.0.1:1/rpc?",
		BuildDir:    buildDir,
		VCSFilePath: filepath.Join(buildDir, "vcs.json"),
		PacmanConf:  absPath,
		Mode:        parser.ModeAUR,
	}
	cmdArgs := parser.MakeArguments()

	run, err := runtime.NewRuntime(cfg, cmdArgs, "1.0.0")
	require.NoError(t, err)

	run.QueryBuilder.Execute(context.Background(), &mock.DBExecutor{}, []string{"yay"})
	assert.Equal(t, 1, run.QueryBuilder.Len())
}
