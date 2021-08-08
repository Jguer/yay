package download

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v10/pkg/settings/exe"
)

func TestGetAURPkgbuild(t *testing.T) {
	pkgBuildHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(gitExtrasPKGBUILD))
	})

	notFoundHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})

	PKGBuild := httptest.NewServer(pkgBuildHandler)
	type args struct {
		handler http.Handler
		pkgName string
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
				handler: pkgBuildHandler,
				pkgName: "git-extras",
			},
			want:    gitExtrasPKGBUILD,
			wantErr: false,
		},
		{
			name: "not found package",
			args: args{
				handler: notFoundHandler,
				pkgName: "git-extras",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AURPackageURL = PKGBuild.URL
			PKGBuild.Config.Handler = tt.args.handler
			got, err := AURPKGBUILD(PKGBuild.Client(), tt.args.pkgName)
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
	cmdRunner := &testRunner{}
	cmdBuilder := &testGitBuilder{
		index: 0,
		test:  t,
		want:  "/usr/local/bin/git --no-replace-objects -C /tmp/doesnt-exist clone --no-progress https://aur.archlinux.org/yay-bin.git yay-bin",
		parentBuilder: &exe.CmdBuilder{
			Runner:   cmdRunner,
			GitBin:   "/usr/local/bin/git",
			GitFlags: []string{"--no-replace-objects"},
		},
	}
	newCloned, err := AURPKGBUILDRepo(cmdBuilder, "https://aur.archlinux.org", "yay-bin", "/tmp/doesnt-exist", false)
	assert.NoError(t, err)
	assert.Equal(t, true, newCloned)
}

// GIVEN a previous existing folder with permissions
// WHEN AURPKGBUILDRepo is called
// THEN a pull command should be formed
func TestAURPKGBUILDRepoExistsPerms(t *testing.T) {
	dir, _ := ioutil.TempDir("/tmp/", "yay-test")
	defer os.RemoveAll(dir)

	os.MkdirAll(filepath.Join(dir, "yay-bin", ".git"), 0o777)

	cmdRunner := &testRunner{}
	cmdBuilder := &testGitBuilder{
		index: 0,
		test:  t,
		want:  fmt.Sprintf("/usr/local/bin/git --no-replace-objects -C %s/yay-bin pull --ff-only", dir),
		parentBuilder: &exe.CmdBuilder{
			Runner:   cmdRunner,
			GitBin:   "/usr/local/bin/git",
			GitFlags: []string{"--no-replace-objects"},
		},
	}
	cloned, err := AURPKGBUILDRepo(cmdBuilder, "https://aur.archlinux.org", "yay-bin", dir, false)
	assert.NoError(t, err)
	assert.Equal(t, false, cloned)
}
