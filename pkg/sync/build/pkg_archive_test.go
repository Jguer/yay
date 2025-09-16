package build

import (
	"context"
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/settings/exe"
)

func TestParsePackageList(t *testing.T) {
	t.Parallel()

	type testCase struct {
		desc           string
		mockStdout     string
		mockStderr     string
		mockErr        error
		wantPkgDests   map[string]string
		wantPkgVersion string
		wantErr        bool
		wantErrText    string // Optional: specific error text to check
	}

	testCases := []testCase{
		{
			desc:       "Standard package",
			mockStdout: "/path/to/package-1.2.3-4-x86_64.pkg.tar.zst\n",
			wantPkgDests: map[string]string{
				"package": "/path/to/package-1.2.3-4-x86_64.pkg.tar.zst",
			},
			wantPkgVersion: "1.2.3-4",
			wantErr:        false,
		},
		{
			desc:       "Package with dash in name",
			mockStdout: "/path/to/package-name-with-dash-1.0.0-1-any.pkg.tar.gz\n",
			wantPkgDests: map[string]string{
				"package-name-with-dash": "/path/to/package-name-with-dash-1.0.0-1-any.pkg.tar.gz",
			},
			wantPkgVersion: "1.0.0-1",
			wantErr:        false, // This should fail with current logic but pass with regex
		},
		{
			desc:       "Multiple packages",
			mockStdout: "/path/to/pkg1-1.0-1-x86_64.pkg.tar.zst\n/other/path/pkg2-2.5-3-any.pkg.tar.xz\n",
			wantPkgDests: map[string]string{
				"pkg1": "/path/to/pkg1-1.0-1-x86_64.pkg.tar.zst",
				"pkg2": "/other/path/pkg2-2.5-3-any.pkg.tar.xz",
			},
			wantPkgVersion: "2.5-3", // Version of the last package processed
			wantErr:        false,
		},
		{
			desc:       "Empty input",
			mockStdout: "",
			wantErr:    true, // Expect NoPkgDestsFoundError
		},
		{
			desc:       "Input with only newline",
			mockStdout: "\n",
			wantErr:    true, // Expect NoPkgDestsFoundError
		},
		{
			desc:       "Makepkg error",
			mockStderr: "makepkg failed",
			mockErr:    fmt.Errorf("exit status 1"),
			wantErr:    true,
		},
		{
			desc:       "Malformed filename (too few dashes)",
			mockStdout: "/path/to/malformed-package.pkg.tar.zst\n",
			wantErr:    true, // Expect "cannot find package name" error
		},
		{
			desc:       "Package with epoch",
			mockStdout: "/path/to/epochpkg-1:2.0.0-1-x86_64.pkg.tar.zst\n",
			wantPkgDests: map[string]string{
				"epochpkg": "/path/to/epochpkg-1:2.0.0-1-x86_64.pkg.tar.zst",
			},
			wantPkgVersion: "1:2.0.0-1",
			wantErr:        false, // This might fail with current logic
		},
		{
			desc:       "Package with .zst extension",
			mockStdout: "/path/to/zstdpkg-3.3-1-any.pkg.tar.zst\n",
			wantPkgDests: map[string]string{
				"zstdpkg": "/path/to/zstdpkg-3.3-1-any.pkg.tar.zst",
			},
			wantPkgVersion: "3.3-1",
			wantErr:        false,
		},
		{
			desc:       "Package with .gz extension",
			mockStdout: "/path/to/gzpkg-3.3-1-any.pkg.tar.gz\n",
			wantPkgDests: map[string]string{
				"gzpkg": "/path/to/gzpkg-3.3-1-any.pkg.tar.gz",
			},
			wantPkgVersion: "3.3-1",
			wantErr:        false,
		},
		{
			desc:       "Package with .xz extension",
			mockStdout: "/path/to/xzpkg-3.3-1-any.pkg.tar.xz\n",
			wantPkgDests: map[string]string{
				"xzpkg": "/path/to/xzpkg-3.3-1-any.pkg.tar.xz",
			},
			wantPkgVersion: "3.3-1",
			wantErr:        false,
		},
		{
			desc:       "Package with .bz2 extension",
			mockStdout: "/path/to/bz2pkg-3.3-1-any.pkg.tar.bz2\n",
			wantPkgDests: map[string]string{
				"bz2pkg": "/path/to/bz2pkg-3.3-1-any.pkg.tar.bz2",
			},
			wantPkgVersion: "3.3-1",
			wantErr:        false,
		},
		{
			desc:       "Package with .tar extension (uncompressed)",
			mockStdout: "/path/to/tarpkg-3.3-1-any.pkg.tar\n",
			wantPkgDests: map[string]string{
				"tarpkg": "/path/to/tarpkg-3.3-1-any.pkg.tar",
			},
			wantPkgVersion: "3.3-1",
			wantErr:        false,
		},
	}

	for _, tc := range testCases {
		// capture range variable
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

			mockRunner := &exe.MockRunner{
				CaptureFn: func(cmd *exec.Cmd) (string, string, error) {
					// Basic check to ensure the command looks right
					require.Contains(t, cmd.String(), "--packagelist")
					return tc.mockStdout, tc.mockStderr, tc.mockErr
				},
			}
			cmdBuilder := &exe.CmdBuilder{Runner: mockRunner} // Simplified for this test

			pkgdests, pkgVersion, err := parsePackageList(context.Background(), cmdBuilder, "/fake/dir")

			if tc.wantErr {
				assert.Error(t, err)
				if tc.wantErrText != "" {
					assert.Contains(t, err.Error(), tc.wantErrText)
				}
				// Check for specific error types if needed
				if tc.desc == "Empty input" || tc.desc == "Input with only newline" {
					assert.IsType(t, &NoPkgDestsFoundError{}, err)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantPkgDests, pkgdests)
				assert.Equal(t, tc.wantPkgVersion, pkgVersion)
			}
		})
	}
}
