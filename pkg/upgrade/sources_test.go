package upgrade

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	aur "github.com/Jguer/aur"
	"github.com/stretchr/testify/assert"

	alpm "github.com/Jguer/go-alpm/v2"

	"github.com/Jguer/yay/v12/pkg/db/mock"
	"github.com/Jguer/yay/v12/pkg/text"
	"github.com/Jguer/yay/v12/pkg/vcs"
)

func Test_upAUR(t *testing.T) {
	t.Parallel()

	type args struct {
		remote          map[string]alpm.IPackage
		aurdata         map[string]*aur.Pkg
		timeUpdate      bool
		enableDowngrade bool
	}
	tests := []struct {
		name string
		args args
		want UpSlice
	}{
		{
			name: "No Updates",
			args: args{
				remote: map[string]alpm.IPackage{
					"hello":     &mock.Package{PName: "hello", PVersion: "2.0.0"},
					"local_pkg": &mock.Package{PName: "local_pkg", PVersion: "1.1.0"},
					"ignored":   &mock.Package{PName: "ignored", PVersion: "1.0.0", PShouldIgnore: true},
				},
				aurdata: map[string]*aur.Pkg{
					"hello":   {Version: "2.0.0", Name: "hello"},
					"ignored": {Version: "2.0.0", Name: "ignored"},
				},
				timeUpdate: false,
			},
			want: UpSlice{Repos: []string{"aur"}, Up: []Upgrade{}},
		},
		{
			name: "Simple Update",
			args: args{
				remote: map[string]alpm.IPackage{
					"hello": &mock.Package{PName: "hello", PVersion: "2.0.0"},
				},
				aurdata:    map[string]*aur.Pkg{"hello": {Version: "2.1.0", Name: "hello"}},
				timeUpdate: false,
			},
			want: UpSlice{Repos: []string{"aur"}, Up: []Upgrade{{Name: "hello", Repository: "aur", LocalVersion: "2.0.0", RemoteVersion: "2.1.0"}}},
		},
		{
			name: "Downgrade",
			args: args{
				remote: map[string]alpm.IPackage{
					"hello": &mock.Package{PName: "hello", PVersion: "2.0.0"},
				},
				aurdata:         map[string]*aur.Pkg{"hello": {Version: "1.0.0", Name: "hello"}},
				timeUpdate:      false,
				enableDowngrade: true,
			},
			want: UpSlice{Repos: []string{"aur"}, Up: []Upgrade{{Name: "hello", Repository: "aur", LocalVersion: "2.0.0", RemoteVersion: "1.0.0"}}},
		},
		{
			name: "Downgrade Disabled",
			args: args{
				remote: map[string]alpm.IPackage{
					"hello": &mock.Package{PName: "hello", PVersion: "2.0.0"},
				},
				aurdata:         map[string]*aur.Pkg{"hello": {Version: "1.0.0", Name: "hello"}},
				timeUpdate:      false,
				enableDowngrade: false,
			},
			want: UpSlice{Repos: []string{"aur"}, Up: []Upgrade{}},
		},
		{
			name: "Mixed Updates Downgrades",
			args: args{
				enableDowngrade: true,
				remote: map[string]alpm.IPackage{
					"up":      &mock.Package{PName: "up", PVersion: "2.0.0"},
					"same":    &mock.Package{PName: "same", PVersion: "3.0.0"},
					"down":    &mock.Package{PName: "down", PVersion: "1.1.0"},
					"ignored": &mock.Package{PName: "ignored", PVersion: "1.0.0", PShouldIgnore: true},
				},
				aurdata: map[string]*aur.Pkg{
					"up":      {Version: "2.1.0", Name: "up"},
					"same":    {Version: "3.0.0", Name: "same"},
					"down":    {Version: "1.0.0", Name: "down"},
					"ignored": {Version: "2.0.0", Name: "ignored"},
				},
				timeUpdate: false,
			},
			want: UpSlice{Repos: []string{"aur"}, Up: []Upgrade{
				{Name: "up", Repository: "aur", LocalVersion: "2.0.0", RemoteVersion: "2.1.0"},
				{Name: "down", Repository: "aur", LocalVersion: "1.1.0", RemoteVersion: "1.0.0"},
			}},
		},
		{
			name: "Time Update",
			args: args{
				remote: map[string]alpm.IPackage{
					"hello": &mock.Package{PName: "hello", PVersion: "2.0.0", PBuildDate: time.Now()},
				},
				aurdata:    map[string]*aur.Pkg{"hello": {Version: "2.0.0", Name: "hello", LastModified: int(time.Now().AddDate(0, 0, 2).Unix())}},
				timeUpdate: true,
			},
			want: UpSlice{Repos: []string{"aur"}, Up: []Upgrade{{Name: "hello", Repository: "aur", LocalVersion: "2.0.0", RemoteVersion: "2.0.0"}}},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := UpAUR(text.NewLogger(io.Discard, strings.NewReader(""), false, "test"),
				tt.args.remote, tt.args.aurdata, tt.args.timeUpdate, tt.args.enableDowngrade)
			assert.EqualValues(t, tt.want, got)
		})
	}
}

func Test_upDevel(t *testing.T) {
	t.Parallel()

	type args struct {
		remote  map[string]alpm.IPackage
		aurdata map[string]*aur.Pkg
		cached  vcs.Store
	}
	tests := []struct {
		name     string
		args     args
		want     UpSlice
		finalLen int
	}{
		{
			name: "No Updates",
			args: args{
				cached: &vcs.Mock{},
				remote: map[string]alpm.IPackage{
					"hello":     &mock.Package{PName: "hello", PVersion: "2.0.0"},
					"local_pkg": &mock.Package{PName: "local_pkg", PVersion: "1.1.0"},
					"ignored":   &mock.Package{PName: "ignored", PVersion: "1.0.0", PShouldIgnore: true},
				},
				aurdata: map[string]*aur.Pkg{
					"hello":   {Version: "2.0.0", Name: "hello"},
					"ignored": {Version: "2.0.0", Name: "ignored"},
				},
			},
			want: UpSlice{Repos: []string{"devel"}},
		},
		{
			name:     "Simple Update",
			finalLen: 3,
			args: args{
				cached: &vcs.Mock{
					ToUpgradeReturn: []string{"hello", "hello4"},
				},
				remote: map[string]alpm.IPackage{
					"hello":  &mock.Package{PName: "hello", PVersion: "2.0.0"},
					"hello2": &mock.Package{PName: "hello2", PVersion: "3.0.0"},
					"hello4": &mock.Package{PName: "hello4", PVersion: "4.0.0"},
				},
				aurdata: map[string]*aur.Pkg{
					"hello":  {Version: "2.0.0", Name: "hello"},
					"hello2": {Version: "2.0.0", Name: "hello2"},
					"hello4": {Version: "2.0.0", Name: "hello4"},
				},
			},
			want: UpSlice{
				Repos: []string{"devel"}, Up: []Upgrade{
					{
						Name:          "hello",
						Repository:    "devel",
						LocalVersion:  "2.0.0",
						RemoteVersion: "latest-commit",
					},
					{
						Name:          "hello4",
						Repository:    "devel",
						LocalVersion:  "4.0.0",
						RemoteVersion: "latest-commit",
					},
				},
			},
		},
		{
			name:     "No update returned",
			finalLen: 1,
			args: args{
				cached: &vcs.Mock{ToUpgradeReturn: []string{}},
				remote: map[string]alpm.IPackage{
					"hello": &mock.Package{PName: "hello", PVersion: "2.0.0"},
				},
				aurdata: map[string]*aur.Pkg{"hello": {Version: "2.0.0", Name: "hello"}},
			},
			want: UpSlice{Repos: []string{"devel"}},
		},
		{
			name:     "No update returned - ignored",
			finalLen: 1,
			args: args{
				cached: &vcs.Mock{
					ToUpgradeReturn: []string{"hello"},
				},
				remote: map[string]alpm.IPackage{
					"hello": &mock.Package{PName: "hello", PVersion: "2.0.0", PShouldIgnore: true},
				},
				aurdata: map[string]*aur.Pkg{"hello": {Version: "2.0.0", Name: "hello"}},
			},
			want: UpSlice{Repos: []string{"devel"}},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := UpDevel(context.Background(),
				text.NewLogger(io.Discard, strings.NewReader(""), false, "test"),
				tt.args.remote, tt.args.aurdata, tt.args.cached)
			assert.ElementsMatch(t, tt.want.Up, got.Up)
		})
	}
}
