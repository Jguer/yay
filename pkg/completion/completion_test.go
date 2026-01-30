//go:build !integration
// +build !integration

package completion

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/db/mock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const samplePackageResp = `
# AUR package list, generated on Fri, 24 Jul 2020 22:05:22 GMT
cytadela
bitefusion
globs-svn
ri-li
globs-benchmarks-svn
dunelegacy
lumina
eternallands-sound
`

const expectPackageCompletion = `cytadela	AUR
bitefusion	AUR
globs-svn	AUR
ri-li	AUR
globs-benchmarks-svn	AUR
dunelegacy	AUR
lumina	AUR
eternallands-sound	AUR
`

type mockDoer struct {
	t                *testing.T
	returnBody       []byte
	returnStatusCode int
	returnErr        error
	wantURL          string
}

func (m *mockDoer) Get(url string) (*http.Response, error) {
	assert.Equal(m.t, m.wantURL, url)
	return &http.Response{
		StatusCode: m.returnStatusCode,
		Body:       io.NopCloser(bytes.NewReader(m.returnBody)),
	}, m.returnErr
}

func gzipString(s string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write([]byte(s))
	gz.Close()
	return buf.Bytes()
}

func Test_createAURList(t *testing.T) {
	t.Parallel()
	doer := &mockDoer{
		t:                t,
		wantURL:          "https://aur.archlinux.org/packages.gz",
		returnStatusCode: 200,
		returnBody:       []byte(samplePackageResp),
		returnErr:        nil,
	}
	out := &bytes.Buffer{}
	err := createAURList(context.Background(), doer, "https://aur.archlinux.org", out, nil)
	assert.NoError(t, err)
	gotOut := out.String()
	assert.Equal(t, expectPackageCompletion, gotOut)
}

func Test_createAURListGzip(t *testing.T) {
	t.Parallel()
	doer := &mockDoer{
		t:                t,
		wantURL:          "https://aur.archlinux.org/packages.gz",
		returnStatusCode: 200,
		returnBody:       gzipString(samplePackageResp),
		returnErr:        nil,
	}
	out := &bytes.Buffer{}
	err := createAURList(context.Background(), doer, "https://aur.archlinux.org", out, nil)
	assert.NoError(t, err)
	gotOut := out.String()
	assert.Equal(t, expectPackageCompletion, gotOut)
}

func Test_createAURListHTTPError(t *testing.T) {
	t.Parallel()
	doer := &mockDoer{
		t:                t,
		wantURL:          "https://aur.archlinux.org/packages.gz",
		returnStatusCode: 200,
		returnBody:       []byte(samplePackageResp),
		returnErr:        errors.New("Not available"),
	}

	out := &bytes.Buffer{}
	err := createAURList(context.Background(), doer, "https://aur.archlinux.org", out, nil)
	assert.EqualError(t, err, "Not available")
}

func Test_createAURListStatusError(t *testing.T) {
	t.Parallel()
	doer := &mockDoer{
		t:                t,
		wantURL:          "https://aur.archlinux.org/packages.gz",
		returnStatusCode: 503,
		returnBody:       []byte(samplePackageResp),
		returnErr:        nil,
	}

	out := &bytes.Buffer{}
	err := createAURList(context.Background(), doer, "https://aur.archlinux.org", out, nil)
	assert.EqualError(t, err, "invalid status code: 503")
}

func TestNeedsUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupFile      func(t *testing.T, path string)
		interval       int
		force          bool
		expectedResult bool
	}{
		{
			name:           "force returns true",
			setupFile:      nil,
			interval:       7,
			force:          true,
			expectedResult: true,
		},
		{
			name:           "file does not exist returns true",
			setupFile:      nil,
			interval:       7,
			force:          false,
			expectedResult: true,
		},
		{
			name: "fresh file returns false",
			setupFile: func(t *testing.T, path string) {
				t.Helper()
				err := os.WriteFile(path, []byte("test"), 0o600)
				require.NoError(t, err)
			},
			interval:       7,
			force:          false,
			expectedResult: false,
		},
		{
			name: "interval -1 never updates",
			setupFile: func(t *testing.T, path string) {
				t.Helper()
				err := os.WriteFile(path, []byte("test"), 0o600)
				require.NoError(t, err)
				// Set file time to 30 days ago
				oldTime := time.Now().Add(-30 * 24 * time.Hour)
				err = os.Chtimes(path, oldTime, oldTime)
				require.NoError(t, err)
			},
			interval:       -1,
			force:          false,
			expectedResult: false,
		},
		{
			name: "old file returns true",
			setupFile: func(t *testing.T, path string) {
				t.Helper()
				err := os.WriteFile(path, []byte("test"), 0o600)
				require.NoError(t, err)
				// Set file time to 10 days ago
				oldTime := time.Now().Add(-10 * 24 * time.Hour)
				err = os.Chtimes(path, oldTime, oldTime)
				require.NoError(t, err)
			},
			interval:       7,
			force:          false,
			expectedResult: true,
		},
		{
			name: "file within interval returns false",
			setupFile: func(t *testing.T, path string) {
				t.Helper()
				err := os.WriteFile(path, []byte("test"), 0o600)
				require.NoError(t, err)
				// Set file time to 3 days ago
				oldTime := time.Now().Add(-3 * 24 * time.Hour)
				err = os.Chtimes(path, oldTime, oldTime)
				require.NoError(t, err)
			},
			interval:       7,
			force:          false,
			expectedResult: false,
		},
		{
			name: "empty file returns true",
			setupFile: func(t *testing.T, path string) {
				t.Helper()
				// Create an empty file
				err := os.WriteFile(path, []byte(""), 0o600)
				require.NoError(t, err)
			},
			interval:       7,
			force:          false,
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			completionPath := filepath.Join(tmpDir, "completion")

			if tt.setupFile != nil {
				tt.setupFile(t, completionPath)
			}

			result := NeedsUpdate(completionPath, tt.interval, tt.force)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

// mockPkgSynchronizer implements PkgSynchronizer for testing.
type mockPkgSynchronizer struct {
	packages []db.IPackage
}

func (m *mockPkgSynchronizer) SyncPackages(...string) []db.IPackage {
	return m.packages
}

func Test_createRepoList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		packages       []db.IPackage
		expectedOutput string
		expectedError  error
	}{
		{
			name:           "empty package list",
			packages:       []db.IPackage{},
			expectedOutput: "",
			expectedError:  nil,
		},
		{
			name: "single package",
			packages: []db.IPackage{
				&mock.Package{PName: "vim", PDB: mock.NewDB("extra")},
			},
			expectedOutput: "vim\textra\n",
			expectedError:  nil,
		},
		{
			name: "multiple packages",
			packages: []db.IPackage{
				&mock.Package{PName: "vim", PDB: mock.NewDB("extra")},
				&mock.Package{PName: "git", PDB: mock.NewDB("extra")},
				&mock.Package{PName: "linux", PDB: mock.NewDB("core")},
			},
			expectedOutput: "vim\textra\ngit\textra\nlinux\tcore\n",
			expectedError:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dbExecutor := &mockPkgSynchronizer{packages: tt.packages}
			out := &bytes.Buffer{}

			err := createRepoList(dbExecutor, out)

			if tt.expectedError != nil {
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedOutput, out.String())
		})
	}
}

// errorWriter is a writer that always returns an error.
type errorWriter struct{}

func (e *errorWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("write error")
}

func Test_createRepoListWriteError(t *testing.T) {
	t.Parallel()

	dbExecutor := &mockPkgSynchronizer{
		packages: []db.IPackage{
			&mock.Package{PName: "vim", PDB: mock.NewDB("extra")},
		},
	}

	err := createRepoList(dbExecutor, &errorWriter{})
	assert.EqualError(t, err, "write error")
}

func TestUpdateCache(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		doer           *mockDoer
		packages       []db.IPackage
		expectedOutput string
		expectError    bool
	}{
		{
			name: "successful update",
			doer: &mockDoer{
				returnStatusCode: 200,
				returnBody:       []byte("# Comment\npkg1\npkg2\n"),
				returnErr:        nil,
			},
			packages: []db.IPackage{
				&mock.Package{PName: "vim", PDB: mock.NewDB("extra")},
			},
			expectedOutput: "pkg1\tAUR\npkg2\tAUR\nvim\textra\n",
			expectError:    false,
		},
		{
			name: "AUR fetch error removes file",
			doer: &mockDoer{
				returnStatusCode: 500,
				returnBody:       []byte{},
				returnErr:        nil,
			},
			packages:    []db.IPackage{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			completionPath := filepath.Join(tmpDir, "subdir", "completion")
			tt.doer.t = t
			tt.doer.wantURL = "https://aur.archlinux.org/packages.gz"

			dbExecutor := &mockPkgSynchronizer{packages: tt.packages}

			err := UpdateCache(context.Background(), tt.doer, dbExecutor, "https://aur.archlinux.org", completionPath, nil)

			if tt.expectError {
				assert.Error(t, err)
				// File should be removed on error
				_, statErr := os.Stat(completionPath)
				assert.True(t, os.IsNotExist(statErr))
			} else {
				require.NoError(t, err)
				content, readErr := os.ReadFile(completionPath)
				require.NoError(t, readErr)
				assert.Equal(t, tt.expectedOutput, string(content))
			}
		})
	}
}

func TestShow(t *testing.T) {
	// Note: Not running in parallel because we need to capture os.Stdout
	tests := []struct {
		name        string
		setupFile   func(t *testing.T, path string)
		doer        *mockDoer
		packages    []db.IPackage
		interval    int
		force       bool
		expectError bool
	}{
		{
			name: "existing fresh file",
			setupFile: func(t *testing.T, path string) {
				t.Helper()
				err := os.WriteFile(path, []byte("cached\tdata\n"), 0o600)
				require.NoError(t, err)
			},
			doer:        nil, // Should not be called
			packages:    nil,
			interval:    7,
			force:       false,
			expectError: false,
		},
		{
			name:      "file needs update",
			setupFile: nil,
			doer: &mockDoer{
				returnStatusCode: 200,
				returnBody:       []byte("# Comment\naur-pkg\n"),
				returnErr:        nil,
			},
			packages: []db.IPackage{
				&mock.Package{PName: "repo-pkg", PDB: mock.NewDB("core")},
			},
			interval:    7,
			force:       false,
			expectError: false,
		},
		{
			name:      "force update",
			setupFile: nil,
			doer: &mockDoer{
				returnStatusCode: 200,
				returnBody:       []byte("# Comment\nforced-pkg\n"),
				returnErr:        nil,
			},
			packages:    []db.IPackage{},
			interval:    7,
			force:       true,
			expectError: false,
		},
		{
			name:      "update cache error",
			setupFile: nil,
			doer: &mockDoer{
				returnStatusCode: 500,
				returnBody:       []byte{},
				returnErr:        nil,
			},
			packages:    []db.IPackage{},
			interval:    7,
			force:       false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Not running in parallel because we capture os.Stdout
			tmpDir := t.TempDir()
			completionPath := filepath.Join(tmpDir, "completion")

			if tt.setupFile != nil {
				tt.setupFile(t, completionPath)
			}

			if tt.doer != nil {
				tt.doer.t = t
				tt.doer.wantURL = "https://aur.archlinux.org/packages.gz"
			}

			dbExecutor := &mockPkgSynchronizer{packages: tt.packages}

			// Capture stdout using a pipe
			oldStdout := os.Stdout
			r, w, pipeErr := os.Pipe()
			require.NoError(t, pipeErr)
			os.Stdout = w

			err := Show(context.Background(), tt.doer, dbExecutor, "https://aur.archlinux.org", completionPath, tt.interval, tt.force, nil)

			// Close writer first, then restore stdout, then read
			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, copyErr := io.Copy(&buf, r)
			r.Close()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NoError(t, copyErr)
				// Verify file exists and has content
				content, readErr := os.ReadFile(completionPath)
				require.NoError(t, readErr)
				assert.NotEmpty(t, content)
			}
		})
	}
}

func TestShowFileOpenError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Use a path that can't be created (directory as file)
	completionPath := filepath.Join(tmpDir, "completion")

	// Create a directory where we expect a file - this will cause OpenFile to fail
	err := os.MkdirAll(completionPath, 0o755)
	require.NoError(t, err)

	doer := &mockDoer{
		t:                t,
		wantURL:          "https://aur.archlinux.org/packages.gz",
		returnStatusCode: 200,
		returnBody:       []byte("# Comment\npkg\n"),
		returnErr:        nil,
	}

	dbExecutor := &mockPkgSynchronizer{packages: []db.IPackage{}}

	err = Show(context.Background(), doer, dbExecutor, "https://aur.archlinux.org", completionPath, 7, true, nil)
	assert.Error(t, err)
}

func TestUpdateCacheMkdirError(t *testing.T) {
	t.Parallel()

	// Create a file where we expect a directory - this will cause MkdirAll to fail
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blocking")
	err := os.WriteFile(blockingFile, []byte("block"), 0o600)
	require.NoError(t, err)

	completionPath := filepath.Join(blockingFile, "subdir", "completion")

	doer := &mockDoer{
		t:                t,
		wantURL:          "https://aur.archlinux.org/packages.gz",
		returnStatusCode: 200,
		returnBody:       []byte("# Comment\npkg\n"),
		returnErr:        nil,
	}

	dbExecutor := &mockPkgSynchronizer{packages: []db.IPackage{}}

	err = UpdateCache(context.Background(), doer, dbExecutor, "https://aur.archlinux.org", completionPath, nil)
	assert.Error(t, err)
}

func Test_createAURListWriteError(t *testing.T) {
	t.Parallel()

	doer := &mockDoer{
		t:                t,
		wantURL:          "https://aur.archlinux.org/packages.gz",
		returnStatusCode: 200,
		returnBody:       []byte("# Comment\npkg1\npkg2\n"),
		returnErr:        nil,
	}

	err := createAURList(context.Background(), doer, "https://aur.archlinux.org", &errorWriter{}, nil)
	assert.EqualError(t, err, "write error")
}
