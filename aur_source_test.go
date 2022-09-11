package main

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v11/pkg/multierror"
	"github.com/Jguer/yay/v11/pkg/settings/exe"
)

type TestMakepkgBuilder struct {
	exe.ICmdBuilder
	parentBuilder *exe.CmdBuilder
	test          *testing.T
	passes        uint32
	want          string
	wantDir       string
	showError     error
}

func (z *TestMakepkgBuilder) BuildMakepkgCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd {
	cmd := z.parentBuilder.BuildMakepkgCmd(ctx, dir, extraArgs...)
	if z.want != "" {
		assert.Contains(z.test, cmd.String(), z.want)
	}
	if z.wantDir != "" {
		assert.Equal(z.test, z.wantDir, cmd.Dir)
	}

	atomic.AddUint32(&z.passes, 1)

	return cmd
}

func (z *TestMakepkgBuilder) Show(cmd *exec.Cmd) error {
	return z.showError
}

// GIVEN 1 package
// WHEN downloadPKGBUILDSource is called
// THEN 1 call should be made to makepkg with the specified parameters and dir
func Test_downloadPKGBUILDSource(t *testing.T) {
	t.Parallel()
	cmdBuilder := &TestMakepkgBuilder{
		parentBuilder: &exe.CmdBuilder{MakepkgConfPath: "/etc/not.conf", MakepkgFlags: []string{"--nocheck"}, MakepkgBin: "makepkg"},
		test:          t,
		want:          "makepkg --nocheck --config /etc/not.conf --verifysource -Ccf",
		wantDir:       "/tmp/yay-bin",
	}
	err := downloadPKGBUILDSource(context.TODO(), cmdBuilder, filepath.Join("/tmp", "yay-bin"), false)
	assert.NoError(t, err)
	assert.Equal(t, 1, int(cmdBuilder.passes))
}

// GIVEN 1 package
// WHEN downloadPKGBUILDSource is called
// THEN 1 call should be made to makepkg which should return error
func Test_downloadPKGBUILDSourceError(t *testing.T) {
	t.Parallel()
	cmdBuilder := &TestMakepkgBuilder{
		parentBuilder: &exe.CmdBuilder{MakepkgConfPath: "/etc/not.conf", MakepkgFlags: []string{"--nocheck"}, MakepkgBin: "makepkg"},
		test:          t,
		want:          "makepkg --nocheck --config /etc/not.conf --verifysource -Ccf",
		wantDir:       "/tmp/yay-bin",
		showError:     &exec.ExitError{},
	}
	err := downloadPKGBUILDSource(context.TODO(), cmdBuilder, filepath.Join("/tmp", "yay-bin"), false)
	assert.Error(t, err)
	assert.EqualError(t, err, "error downloading sources: \x1b[36myay-bin\x1b[0m \n\t context: <nil> \n\t \n")
}

// GIVEN 5 packages
// WHEN downloadPKGBUILDSourceFanout is called
// THEN 5 calls should be made to makepkg
func Test_downloadPKGBUILDSourceFanout(t *testing.T) {
	t.Parallel()

	pkgBuildDirs := []string{"/tmp/yay", "/tmp/yay-bin", "/tmp/yay-git", "/tmp/yay-v11", "/tmp/yay-v12"}
	for _, maxConcurrentDownloads := range []int{0, 3} {
		t.Run(fmt.Sprintf("maxconcurrentdownloads set to %d", maxConcurrentDownloads), func(t *testing.T) {
			cmdBuilder := &TestMakepkgBuilder{
				parentBuilder: &exe.CmdBuilder{
					MakepkgConfPath: "/etc/not.conf",
					MakepkgFlags:    []string{"--nocheck"}, MakepkgBin: "makepkg",
				},
				test: t,
			}

			err := downloadPKGBUILDSourceFanout(context.TODO(), cmdBuilder, pkgBuildDirs, false, maxConcurrentDownloads)
			assert.NoError(t, err)
			assert.Equal(t, 5, int(cmdBuilder.passes))
		})
	}
}

// GIVEN 1 package
// WHEN downloadPKGBUILDSourceFanout is called
// THEN 1 calls should be made to makepkg without concurrency
func Test_downloadPKGBUILDSourceFanoutNoCC(t *testing.T) {
	t.Parallel()
	cmdBuilder := &TestMakepkgBuilder{
		parentBuilder: &exe.CmdBuilder{
			MakepkgConfPath: "/etc/not.conf",
			MakepkgFlags:    []string{"--nocheck"}, MakepkgBin: "makepkg",
		},
		test: t,
	}

	pkgBuildDirs := []string{"/tmp/yay"}

	err := downloadPKGBUILDSourceFanout(context.TODO(), cmdBuilder, pkgBuildDirs, false, 0)
	assert.NoError(t, err)
	assert.Equal(t, 1, int(cmdBuilder.passes))
}

// GIVEN 5 packages
// WHEN downloadPKGBUILDSourceFanout is called
// THEN 5 calls should be made to makepkg
func Test_downloadPKGBUILDSourceFanoutError(t *testing.T) {
	t.Parallel()
	cmdBuilder := &TestMakepkgBuilder{
		parentBuilder: &exe.CmdBuilder{
			MakepkgConfPath: "/etc/not.conf",
			MakepkgFlags:    []string{"--nocheck"}, MakepkgBin: "makepkg",
		},
		test:      t,
		showError: &exec.ExitError{},
	}

	pkgBuildDirs := []string{
		"/tmp/yay",
		"/tmp/yay-bin",
		"/tmp/yay-git",
		"/tmp/yay-v11",
		"/tmp/yay-v12",
	}

	err := downloadPKGBUILDSourceFanout(context.TODO(), cmdBuilder, pkgBuildDirs, false, 0)
	assert.Error(t, err)
	assert.Equal(t, 5, int(cmdBuilder.passes))
	assert.Len(t, err.(*multierror.MultiError).Errors, 5)
}
