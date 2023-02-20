package query

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Jguer/yay/v11/pkg/db/mock"
	"github.com/Jguer/yay/v11/pkg/text"

	"github.com/Jguer/aur"
)

var (
	pkgA = aur.Pkg{
		Name:        "package-a",
		Version:     "1.0.0",
		Description: "Package A description",
		Maintainer:  "Package A Maintainer",
	}
	pkgARepo = &mock.Package{
		PName:        pkgA.Name,
		PVersion:     pkgA.Version,
		PDescription: pkgA.Description,
		PSize:        1,
		PISize:       1,
		PDB:          mock.NewDB("dba"),
	}

	pkgB = aur.Pkg{
		Name:        "package-b",
		Version:     "1.0.0",
		Description: "Package B description",
		Maintainer:  "Package B Maintainer",
	}
	pkgBRepo = &mock.Package{
		PName:        pkgB.Name,
		PVersion:     pkgB.Version,
		PDescription: pkgB.Description,
		PSize:        1,
		PISize:       1,
		PDB:          mock.NewDB("dbb"),
	}
)

func Test_aurQuery_printSearch(t *testing.T) {
	type args struct {
		searchMode        SearchVerbosity
		singleLineResults bool
	}
	tests := []struct {
		name     string
		q        aurQuery
		args     args
		useColor bool
		want     string
	}{
		{
			name: "AUR,Minimal,NoColor",
			q:    aurQuery{pkgA, pkgB},
			args: args{
				searchMode: Minimal,
			},
			want: "package-a\npackage-b\n",
		},
		{
			name: "AUR,DoubleLine,NumberMenu,NoColor",
			q:    aurQuery{pkgA, pkgB},
			args: args{
				searchMode:        NumberMenu,
				singleLineResults: false,
			},
			want: "1 aur/package-a 1.0.0 (+0 0.00) \n    Package A description\n2 aur/package-b 1.0.0 (+0 0.00) \n    Package B description\n",
		},
		{
			name: "AUR,SingleLine,NumberMenu,NoColor",
			q:    aurQuery{pkgA, pkgB},
			args: args{
				searchMode:        NumberMenu,
				singleLineResults: true,
			},
			want: "1 aur/package-a 1.0.0 (+0 0.00) \tPackage A description\n2 aur/package-b 1.0.0 (+0 0.00) \tPackage B description\n",
		},
		{
			name: "AUR,DoubleLine,Detailed,NoColor",
			q:    aurQuery{pkgA, pkgB},
			args: args{
				searchMode:        Detailed,
				singleLineResults: false,
			},
			want: "aur/package-a 1.0.0 (+0 0.00) \n    Package A description\naur/package-b 1.0.0 (+0 0.00) \n    Package B description\n",
		},
		{
			name: "AUR,SingleLine,Detailed,NoColor",
			q:    aurQuery{pkgA, pkgB},
			args: args{
				searchMode:        Detailed,
				singleLineResults: true,
			},
			want: "aur/package-a 1.0.0 (+0 0.00) \tPackage A description\naur/package-b 1.0.0 (+0 0.00) \tPackage B description\n",
		},
		{
			name: "AUR,DoubleLine,Detailed,Color",
			q:    aurQuery{pkgA, pkgB},
			args: args{
				searchMode:        Detailed,
				singleLineResults: false,
			},
			useColor: true,
			want:     "\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mpackage-a\x1b[0m \x1b[36m1.0.0\x1b[0m\x1b[1m (+0\x1b[0m \x1b[1m0.00) \x1b[0m\n    Package A description\n\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mpackage-b\x1b[0m \x1b[36m1.0.0\x1b[0m\x1b[1m (+0\x1b[0m \x1b[1m0.00) \x1b[0m\n    Package B description\n",
		},
		{
			name: "AUR,SingleLine,Detailed,Color",
			q:    aurQuery{pkgA, pkgB},
			args: args{
				searchMode:        Detailed,
				singleLineResults: true,
			},
			useColor: true,
			want:     "\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mpackage-a\x1b[0m \x1b[36m1.0.0\x1b[0m\x1b[1m (+0\x1b[0m \x1b[1m0.00) \x1b[0m\tPackage A description\n\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mpackage-b\x1b[0m \x1b[36m1.0.0\x1b[0m\x1b[1m (+0\x1b[0m \x1b[1m0.00) \x1b[0m\tPackage B description\n",
		},
		{
			name: "AUR,NoPackages",
			q:    aurQuery{},
			args: args{
				searchMode:        Detailed,
				singleLineResults: true,
			},
			useColor: true,
			want:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &strings.Builder{}
			executor := &mock.DBExecutor{LocalPackageFn: func(string) mock.IPackage { return nil }}
			text.UseColor = tt.useColor

			// Fire
			tt.q.printSearch(w, 1, executor, tt.args.searchMode, false, tt.args.singleLineResults)

			got := w.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_repoQuery_printSearch(t *testing.T) {
	type args struct {
		searchMode        SearchVerbosity
		singleLineResults bool
	}
	tests := []struct {
		name     string
		q        repoQuery
		args     args
		useColor bool
		want     string
	}{
		{
			name: "REPO,Minimal,NoColor",
			q:    repoQuery{pkgARepo, pkgBRepo},
			args: args{
				searchMode: Minimal,
			},
			want: "package-a\npackage-b\n",
		},
		{
			name: "REPO,DoubleLine,NumberMenu,NoColor",
			q:    repoQuery{pkgARepo, pkgBRepo},
			args: args{
				searchMode:        NumberMenu,
				singleLineResults: false,
			},
			want: "1 dba/package-a 1.0.0 (1.0 B 1.0 B) \n    Package A description\n2 dbb/package-b 1.0.0 (1.0 B 1.0 B) \n    Package B description\n",
		},
		{
			name: "REPO,SingleLine,NumberMenu,NoColor",
			q:    repoQuery{pkgARepo, pkgBRepo},
			args: args{
				searchMode:        NumberMenu,
				singleLineResults: true,
			},
			want: "1 dba/package-a 1.0.0 (1.0 B 1.0 B) \tPackage A description\n2 dbb/package-b 1.0.0 (1.0 B 1.0 B) \tPackage B description\n",
		},
		{
			name: "REPO,DoubleLine,Detailed,NoColor",
			q:    repoQuery{pkgARepo, pkgBRepo},
			args: args{
				searchMode:        Detailed,
				singleLineResults: false,
			},
			want: "dba/package-a 1.0.0 (1.0 B 1.0 B) \n    Package A description\ndbb/package-b 1.0.0 (1.0 B 1.0 B) \n    Package B description\n",
		},
		{
			name: "REPO,SingleLine,Detailed,NoColor",
			q:    repoQuery{pkgARepo, pkgBRepo},
			args: args{
				searchMode:        Detailed,
				singleLineResults: true,
			},
			want: "dba/package-a 1.0.0 (1.0 B 1.0 B) \tPackage A description\ndbb/package-b 1.0.0 (1.0 B 1.0 B) \tPackage B description\n",
		},
		{
			name: "AUR,DoubleLine,Detailed,Color",
			q:    repoQuery{pkgARepo, pkgBRepo},
			args: args{
				searchMode:        Detailed,
				singleLineResults: false,
			},
			useColor: true,
			want:     "\x1b[1m\x1b[35mdba\x1b[0m\x1b[0m/\x1b[1mpackage-a\x1b[0m \x1b[36m1.0.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    Package A description\n\x1b[1m\x1b[36mdbb\x1b[0m\x1b[0m/\x1b[1mpackage-b\x1b[0m \x1b[36m1.0.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    Package B description\n",
		},
		{
			name: "REPO,SingleLine,Detailed,Color",
			q:    repoQuery{pkgARepo, pkgBRepo},
			args: args{
				searchMode:        Detailed,
				singleLineResults: true,
			},
			useColor: true,
			want:     "\x1b[1m\x1b[35mdba\x1b[0m\x1b[0m/\x1b[1mpackage-a\x1b[0m \x1b[36m1.0.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\tPackage A description\n\x1b[1m\x1b[36mdbb\x1b[0m\x1b[0m/\x1b[1mpackage-b\x1b[0m \x1b[36m1.0.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\tPackage B description\n",
		},
		{
			name: "REPO,NoPackages",
			q:    repoQuery{},
			args: args{
				searchMode:        Detailed,
				singleLineResults: true,
			},
			useColor: true,
			want:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &strings.Builder{}
			executor := &mock.DBExecutor{LocalPackageFn: func(string) mock.IPackage { return nil }}
			text.UseColor = tt.useColor

			// Fire
			tt.q.printSearch(w, executor, tt.args.searchMode, false, tt.args.singleLineResults)

			got := w.String()
			assert.Equal(t, tt.want, got)
		})
	}
}
