package upgrade

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"testing"
	"time"

	aur "github.com/Jguer/aur"
	"github.com/stretchr/testify/assert"

	alpm "github.com/Jguer/go-alpm/v2"

	"github.com/Jguer/yay/v11/pkg/db/mock"
	"github.com/Jguer/yay/v11/pkg/vcs"
)

func Test_upAUR(t *testing.T) {
	t.Parallel()

	type args struct {
		remote     []alpm.IPackage
		aurdata    map[string]*aur.Pkg
		timeUpdate bool
	}
	tests := []struct {
		name string
		args args
		want UpSlice
	}{
		{
			name: "No Updates",
			args: args{
				remote: []alpm.IPackage{
					&mock.Package{PName: "hello", PVersion: "2.0.0"},
					&mock.Package{PName: "local_pkg", PVersion: "1.1.0"},
					&mock.Package{PName: "ignored", PVersion: "1.0.0", PShouldIgnore: true},
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
				remote:     []alpm.IPackage{&mock.Package{PName: "hello", PVersion: "2.0.0"}},
				aurdata:    map[string]*aur.Pkg{"hello": {Version: "2.1.0", Name: "hello"}},
				timeUpdate: false,
			},
			want: UpSlice{Repos: []string{"aur"}, Up: []Upgrade{{Name: "hello", Repository: "aur", LocalVersion: "2.0.0", RemoteVersion: "2.1.0"}}},
		},
		{
			name: "Time Update",
			args: args{
				remote:     []alpm.IPackage{&mock.Package{PName: "hello", PVersion: "2.0.0", PBuildDate: time.Now()}},
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
			got := UpAUR(tt.args.remote, tt.args.aurdata, tt.args.timeUpdate)
			assert.EqualValues(t, tt.want, got)
		})
	}
}

type MockRunner struct {
	Returned []string
	Index    int
	t        *testing.T
}

func (r *MockRunner) Show(cmd *exec.Cmd) error {
	return nil
}

func (r *MockRunner) Capture(cmd *exec.Cmd) (stdout, stderr string, err error) {
	i, _ := strconv.Atoi(cmd.Args[len(cmd.Args)-1])
	if i >= len(r.Returned) {
		fmt.Println(r.Returned)
		fmt.Println(cmd.Args)
		fmt.Println(i)
	}
	stdout = r.Returned[i]
	assert.Contains(r.t, cmd.Args, "ls-remote")
	return stdout, stderr, err
}

func Test_upDevel(t *testing.T) {
	t.Parallel()

	type args struct {
		remote  []alpm.IPackage
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
				remote: []alpm.IPackage{
					&mock.Package{PName: "hello", PVersion: "2.0.0"},
					&mock.Package{PName: "local_pkg", PVersion: "1.1.0"},
					&mock.Package{PName: "ignored", PVersion: "1.0.0", PShouldIgnore: true},
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
				remote: []alpm.IPackage{
					&mock.Package{PName: "hello", PVersion: "2.0.0"},
					&mock.Package{PName: "hello2", PVersion: "3.0.0"},
					&mock.Package{PName: "hello4", PVersion: "4.0.0"},
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
				cached:  &vcs.Mock{ToUpgradeReturn: []string{}},
				remote:  []alpm.IPackage{&mock.Package{PName: "hello", PVersion: "2.0.0"}},
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
				remote:  []alpm.IPackage{&mock.Package{PName: "hello", PVersion: "2.0.0", PShouldIgnore: true}},
				aurdata: map[string]*aur.Pkg{"hello": {Version: "2.0.0", Name: "hello"}},
			},
			want: UpSlice{Repos: []string{"devel"}},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := UpDevel(context.TODO(), tt.args.remote, tt.args.aurdata, tt.args.cached)
			assert.ElementsMatch(t, tt.want.Up, got.Up)
		})
	}
}
