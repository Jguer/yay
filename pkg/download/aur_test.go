package download

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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
			got, err := GetAURPkgbuild(PKGBuild.Client(), tt.args.pkgName)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, string(got))
		})
	}
}
