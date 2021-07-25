package download

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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
		t.Run(tt.name, func(t *testing.T) {
			got, err := getPackageURL(tt.args.db, tt.args.pkgName)
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrInvalidRepository)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetABSPkgbuild(t *testing.T) {
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
		dbName  string
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
				dbName:  "core",
				pkgName: "git-extras",
			},
			want:    gitExtrasPKGBUILD,
			wantErr: false,
		},
		{
			name: "not found package",
			args: args{
				handler: notFoundHandler,
				dbName:  "core",
				pkgName: "git-extras",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ABSPackageURL = PKGBuild.URL
			PKGBuild.Config.Handler = tt.args.handler
			got, err := GetABSPkgbuild(PKGBuild.Client(), tt.args.dbName, tt.args.pkgName)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, string(got))
		})
	}
}
