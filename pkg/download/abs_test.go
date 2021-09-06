package download

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v10/pkg/settings/exe"
)

const gitExtrasPKGBUILD = `pkgname=git-extras
pkgver=6.1.0
pkgrel=1
pkgdesc="GIT utilities -- repo summary, commit counting, repl, changelog population and more"
arch=('any')
url="https://github.com/tj/${pkgname}"
license=('MIT')
depends=('git')
source=("${pkgname}-${pkgver}.tar.gz::${url}/archive/${pkgver}.tar.gz")
sha256sums=('7be0b15ee803d76d2c2e8036f5d9db6677f2232bb8d2c4976691ff7ae026a22f')
b2sums=('3450edecb3116e19ffcf918b118aee04f025c06d812e29e8701f35a3c466b13d2578d41c8e1ee93327743d0019bf98bb3f397189e19435f89e3a259ff1b82747')

package() {
    cd "${srcdir}/${pkgname}-${pkgver}"

    # avoid annoying interactive prompts if an alias is in your gitconfig
    export GIT_CONFIG=/dev/null
    make DESTDIR="${pkgdir}" PREFIX=/usr SYSCONFDIR=/etc install
    install -Dm644 LICENSE "${pkgdir}/usr/share/licenses/${pkgname}/LICENSE"
}`

func Test_getPackageURL(t *testing.T) {
	t.Parallel()
	type args struct {
		db      string
		pkgName string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "community package",
			args: args{
				db:      "community",
				pkgName: "kitty",
			},
			want:    "https://github.com/archlinux/svntogit-community/raw/packages/kitty/trunk/PKGBUILD",
			wantErr: false,
		},
		{
			name: "core package",
			args: args{
				db:      "core",
				pkgName: "linux",
			},
			want:    "https://github.com/archlinux/svntogit-packages/raw/packages/linux/trunk/PKGBUILD",
			wantErr: false,
		},
		{
			name: "personal repo package",
			args: args{
				db:      "sweswe",
				pkgName: "linux",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := getPackageURL(tt.args.db, tt.args.pkgName)
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrInvalidRepository)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetABSPkgbuild(t *testing.T) {
	t.Parallel()

	type args struct {
		dbName  string
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
				dbName:  "core",
				body:    gitExtrasPKGBUILD,
				status:  200,
				pkgName: "git-extras",
				wantURL: "https://github.com/archlinux/svntogit-packages/raw/packages/git-extras/trunk/PKGBUILD",
			},
			want:    gitExtrasPKGBUILD,
			wantErr: false,
		},
		{
			name: "not found package",
			args: args{
				dbName:  "core",
				body:    "",
				status:  404,
				pkgName: "git-git",
				wantURL: "https://github.com/archlinux/svntogit-packages/raw/packages/git-git/trunk/PKGBUILD",
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
			got, err := ABSPKGBUILD(httpClient, tt.args.dbName, tt.args.pkgName)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, string(got))
		})
	}
}

func Test_getPackageRepoURL(t *testing.T) {
	t.Parallel()

	type args struct {
		db string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "community package",
			args:    args{db: "community"},
			want:    "https://github.com/archlinux/svntogit-community.git",
			wantErr: false,
		},
		{
			name:    "core package",
			args:    args{db: "core"},
			want:    "https://github.com/archlinux/svntogit-packages.git",
			wantErr: false,
		},
		{
			name:    "personal repo package",
			args:    args{db: "sweswe"},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := getPackageRepoURL(tt.args.db)
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrInvalidRepository)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// GIVEN no previous existing folder
// WHEN ABSPKGBUILDRepo is called
// THEN a clone command should be formed
func TestABSPKGBUILDRepo(t *testing.T) {
	t.Parallel()
	cmdRunner := &testRunner{}
	want := "/usr/local/bin/git --no-replace-objects -C /tmp/doesnt-exist clone --no-progress --single-branch -b packages/linux https://github.com/archlinux/svntogit-packages.git linux"
	if os.Getuid() == 0 {
		ld := "systemd-run"
		if path, _ := exec.LookPath(ld); path != "" {
			ld = path
		}
		want = fmt.Sprintf("%s --service-type=oneshot --pipe --wait --pty -p DynamicUser=yes -p CacheDirectory=yay -E HOME=/tmp  --no-replace-objects -C /tmp/doesnt-exist clone --no-progress --single-branch -b packages/linux https://github.com/archlinux/svntogit-packages.git linux", ld)
	}

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
	newClone, err := ABSPKGBUILDRepo(context.TODO(), cmdBuilder, "core", "linux", "/tmp/doesnt-exist", false)
	assert.NoError(t, err)
	assert.Equal(t, true, newClone)
}

// GIVEN a previous existing folder with permissions
// WHEN ABSPKGBUILDRepo is called
// THEN a pull command should be formed
func TestABSPKGBUILDRepoExistsPerms(t *testing.T) {
	t.Parallel()
	dir, _ := os.MkdirTemp("/tmp/", "yay-test")
	defer os.RemoveAll(dir)

	os.MkdirAll(filepath.Join(dir, "linux", ".git"), 0o777)

	want := fmt.Sprintf("/usr/local/bin/git --no-replace-objects -C %s/linux pull --ff-only", dir)
	if os.Getuid() == 0 {
		ld := "systemd-run"
		if path, _ := exec.LookPath(ld); path != "" {
			ld = path
		}
		want = fmt.Sprintf("%s --service-type=oneshot --pipe --wait --pty -p DynamicUser=yes -p CacheDirectory=yay -E HOME=/tmp  --no-replace-objects -C %s/linux pull --ff-only", ld, dir)
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
	newClone, err := ABSPKGBUILDRepo(context.TODO(), cmdBuilder, "core", "linux", dir, false)
	assert.NoError(t, err)
	assert.Equal(t, false, newClone)
}
