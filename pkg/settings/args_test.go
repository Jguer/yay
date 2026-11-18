//go:build !integration

package settings

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/settings/parser"
)

func TestConfiguration_ParseCommandLineNoUpgradeMenu(t *testing.T) {
	originalArgs := os.Args
	t.Cleanup(func() { os.Args = originalArgs })

	tests := []struct {
		name  string
		arg   string
		start bool
		want  bool
	}{
		{name: "enable", arg: "--noupgrademenu", start: false, want: true},
		{name: "disable", arg: "--noupgrademenu=false", start: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = []string{"yay", tt.arg}
			cfg := &Configuration{NoUpgradeMenu: tt.start}
			args := parser.MakeArguments()

			require.NoError(t, cfg.ParseCommandLine(args))
			assert.Equal(t, tt.want, cfg.NoUpgradeMenu)
			assert.NotContains(t, args.Options, "noupgrademenu")
		})
	}
}

func TestDefaultConfigShowsUpgradeMenu(t *testing.T) {
	t.Parallel()

	assert.False(t, DefaultConfig("test").NoUpgradeMenu)
}
