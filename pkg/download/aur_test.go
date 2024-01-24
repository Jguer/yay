//go:build !integration
// +build !integration

package download

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v12/pkg/settings/exe"
)

func TestGetAURPkgbuild(t *testing.T) {
	t.Parallel()

	type args struct {
		body    string
		status  int
		pkgName string
		wantURL string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "found package",
			args: args{
				body:    gitExtrasPKGBUILD,
				status:  200,
				pkgName: "git-extras",
				wantURL: "https://aur.archlinux.org/cgit/aur.git/plain/PKGBUILD?h=git-extras",
			},
			want:    gitExtrasPKGBUILD,
			wantErr: false,
		},
		{
			name: "not found package",
			args: args{
				body:    "",
				status:  404,
				pkgName: "git-git",
				wantURL: "https://aur.archlinux.org/cgit/aur.git/plain/PKGBUILD?h=git-git",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			httpClient := &testClient{
				t:       t,
				wantURL: tt.args.wantURL,
				body:    tt.args.body,
				status:  tt.args.status,
			}
			got, err := AURPKGBUILD(httpClient, tt.args.pkgName, "https://aur.archlinux.org")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, string(got))
		})
	}
}

// GIVEN no previous existing folder
// WHEN AURPKGBUILDRepo is called
// THEN a clone command should be formed
func TestAURPKGBUILDRepo(t *testing.T) {
	t.Parallel()
	want := "/usr/local/bin/git --no-replace-objects -C /tmp/doesnt-exist clone --no-progress --depth 1 https://aur.archlinux.org/yay-bin.git yay-bin"
	if os.Getuid() == 0 {
		ld := "systemd-run"
		if path, _ := exec.LookPath(ld); path != "" {
			ld = path
		}
		want = fmt.Sprintf("%s --service-type=oneshot --pipe --wait --pty --quiet -p DynamicUser=yes -p CacheDirectory=yay -E HOME=/tmp  --no-replace-objects -C /tmp/doesnt-exist clone --no-progress --depth 1 https://aur.archlinux.org/yay-bin.git yay-bin", ld)
	}

	cmdRunner := &testRunner{}
	cmdBuilder := &testGitBuilder{
		index: 0,
		test:  t,
		want:  want,
		parentBuilder: &exe.CmdBuilder{
			Runner:   cmdRunner,
			GitBin:   "/usr/local/bin/git",
			GitFlags: []string{"--no-replace-objects"},
		},
	}
	newCloned, err := AURPKGBUILDRepo(context.Background(), cmdBuilder, "https://aur.archlinux.org", "yay-bin", "/tmp/doesnt-exist", false)
	assert.NoError(t, err)
	assert.Equal(t, true, newCloned)
}

// GIVEN a previous existing folder with permissions
// WHEN AURPKGBUILDRepo is called
// THEN a pull command should be formed
func TestAURPKGBUILDRepoExistsPerms(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "yay-bin", ".git"), 0o777)

	want := fmt.Sprintf("/usr/local/bin/git --no-replace-objects -C %s/yay-bin pull --rebase --autostash", dir)
	if os.Getuid() == 0 {
		ld := "systemd-run"
		if path, _ := exec.LookPath(ld); path != "" {
			ld = path
		}
		want = fmt.Sprintf("%s --service-type=oneshot --pipe --wait --pty --quiet -p DynamicUser=yes -p CacheDirectory=yay -E HOME=/tmp  --no-replace-objects -C %s/yay-bin pull --rebase --autostash", ld, dir)
	}

	cmdRunner := &testRunner{}
	cmdBuilder := &testGitBuilder{
		index: 0,
		test:  t,
		want:  want,
		parentBuilder: &exe.CmdBuilder{
			Runner:   cmdRunner,
			GitBin:   "/usr/local/bin/git",
			GitFlags: []string{"--no-replace-objects"},
		},
	}
	cloned, err := AURPKGBUILDRepo(context.Background(), cmdBuilder, "https://aur.archlinux.org", "yay-bin", dir, false)
	assert.NoError(t, err)
	assert.Equal(t, false, cloned)
}

func TestAURPKGBUILDRepos(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "yay-bin", ".git"), 0o777)

	targets := []string{"yay", "yay-bin", "yay-git"}
	cmdRunner := &testRunner{}
	cmdBuilder := &testGitBuilder{
		index: 0,
		test:  t,
		want:  "",
		parentBuilder: &exe.CmdBuilder{
			Runner:   cmdRunner,
			GitBin:   "/usr/local/bin/git",
			GitFlags: []string{},
		},
	}
	cloned, err := AURPKGBUILDRepos(context.Background(), cmdBuilder, newTestLogger(), targets, "https://aur.archlinux.org", dir, false)

	assert.NoError(t, err)
	assert.EqualValues(t, map[string]bool{"yay": true, "yay-bin": false, "yay-git": true}, cloned)
}
