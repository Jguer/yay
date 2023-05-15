package settings_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
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
		Runtime:     &settings.Runtime{Logger: text.NewLogger(nil, nil, nil, false, "")},
	}
	cmdArgs := parser.MakeArguments()
	version := "1.0.0"

	// Call the function being tested
	runtime, err := settings.BuildRuntime(cfg, cmdArgs, version)

	// Assert the function's output
	assert.NotNil(t, runtime)
	assert.Nil(t, err)
	assert.Nil(t, runtime.QueryBuilder)
	assert.Nil(t, runtime.PacmanConf)
	assert.NotNil(t, runtime.VCSStore)
	assert.NotNil(t, runtime.CmdBuilder)
	assert.NotNil(t, runtime.HTTPClient)
	assert.NotNil(t, runtime.AURClient)
	assert.NotNil(t, runtime.VoteClient)
	assert.NotNil(t, runtime.AURCache)
	assert.Nil(t, runtime.DBExecutor)
	assert.NotNil(t, runtime.Logger)
}
