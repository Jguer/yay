package settings

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v11/pkg/text"
)

func TestMigrationNothingToDo(t *testing.T) {
	t.Parallel()
	// Create temporary file for config
	configFile, err := os.CreateTemp("/tmp", "yay-*-config.json")
	require.NoError(t, err)

	testFilePath := configFile.Name()
	defer os.Remove(testFilePath)
	// Create config with configVersion
	config := Configuration{
		Version: "99.0.0",
		// Create runtime with runtimeVersion
		Runtime: &Runtime{
			Version: "20.0.0",
			Logger:  text.NewLogger(io.Discard, strings.NewReader(""), false, "test"),
		},
	}

	// Run Migration
	err = config.RunMigrations(DefaultMigrations(), testFilePath)
	require.NoError(t, err)

	// Check file contents if wantSave otherwise check file empty
	cfile, err := os.Open(testFilePath)
	require.NoError(t, err)
	defer cfile.Close()

	decoder := json.NewDecoder(cfile)
	newConfig := Configuration{}
	err = decoder.Decode(&newConfig)
	require.Error(t, err)
	assert.Empty(t, newConfig.Version)
}

func TestProvidesMigrationDo(t *testing.T) {
	migration := &configProviderMigration{}
	config := Configuration{
		Provides: true,
		Runtime: &Runtime{
			Logger: text.NewLogger(io.Discard, strings.NewReader(""), false, "test"),
		},
	}

	assert.True(t, migration.Do(&config))

	falseConfig := Configuration{Provides: false}

	assert.False(t, migration.Do(&falseConfig))
}

func TestProvidesMigration(t *testing.T) {
	t.Parallel()
	type testCase struct {
		desc       string
		testConfig *Configuration
		wantSave   bool
	}

	testCases := []testCase{
		{
			desc: "to upgrade",
			testConfig: &Configuration{
				Version:  "11.0.1",
				Runtime:  &Runtime{Version: "11.2.1"},
				Provides: true,
			},
			wantSave: true,
		},
		{
			desc: "to upgrade-git",
			testConfig: &Configuration{
				Version:  "11.2.0.r7.g6f60892",
				Runtime:  &Runtime{Version: "11.2.1"},
				Provides: true,
			},
			wantSave: true,
		},
		{
			desc: "to not upgrade",
			testConfig: &Configuration{
				Version:  "11.2.0",
				Runtime:  &Runtime{Version: "11.2.1"},
				Provides: false,
			},
			wantSave: false,
		},
		{
			desc: "to not upgrade - target version",
			testConfig: &Configuration{
				Version:  "11.2.1",
				Runtime:  &Runtime{Version: "11.2.1"},
				Provides: true,
			},
			wantSave: false,
		},
		{
			desc: "to not upgrade - new version",
			testConfig: &Configuration{
				Version:  "11.3.0",
				Runtime:  &Runtime{Version: "11.3.0"},
				Provides: true,
			},
			wantSave: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Create temporary file for config
			configFile, err := os.CreateTemp("/tmp", "yay-*-config.json")
			require.NoError(t, err)

			testFilePath := configFile.Name()
			defer os.Remove(testFilePath)
			// Create config with configVersion and provides
			tcConfig := Configuration{
				Version:  tc.testConfig.Version,
				Provides: tc.testConfig.Provides,
				// Create runtime with runtimeVersion
				Runtime: &Runtime{
					Logger:  text.NewLogger(io.Discard, strings.NewReader(""), false, "test"),
					Version: tc.testConfig.Runtime.Version,
				},
			}

			// Run Migration
			err = tcConfig.RunMigrations(
				[]configMigration{&configProviderMigration{}},
				testFilePath)

			require.NoError(t, err)

			// Check file contents if wantSave otherwise check file empty
			cfile, err := os.Open(testFilePath)
			require.NoError(t, err)
			defer cfile.Close()

			decoder := json.NewDecoder(cfile)
			newConfig := Configuration{}
			err = decoder.Decode(&newConfig)
			if tc.wantSave {
				require.NoError(t, err)
				assert.Equal(t, tc.testConfig.Runtime.Version, newConfig.Version)
				assert.Equal(t, false, newConfig.Provides)
			} else {
				require.Error(t, err)
				assert.Empty(t, newConfig.Version)
			}
		})
	}
}
