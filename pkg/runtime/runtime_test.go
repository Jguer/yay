//go:build !integration
// +build !integration

package runtime_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/runtime"
	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
)

func TestBuildRuntime(t *testing.T) {
	t.Parallel()
	// Prepare test inputs
	cfg := &settings.Configuration{
		Debug:       true,
		UseRPC:      false,
		AURURL:      "https://aur.archlinux.org",
		AURRPCURL:   "https://aur.archlinux.org/rpc",
		BuildDir:    "/tmp",
		VCSFilePath: "",
		PacmanConf:  "../../testdata/pacman.conf",
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
