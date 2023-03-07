package query

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Jguer/aur/rpc"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/db/mock"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"

	"github.com/Jguer/go-alpm/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validPayload = `{
	"resultcount": 1,
	"results": [
		{
			"Description": "The Linux-ck kernel and modules with ck's hrtimer patches",
			"FirstSubmitted": 1311346274,
			"ID": 1045311,
			"LastModified": 1646250901,
			"Maintainer": "graysky",
			"Name": "linux-ck",
			"NumVotes": 450,
			"OutOfDate": null,
			"PackageBase": "linux-ck",
			"PackageBaseID": 50911,
			"Popularity": 1.511141,
			"URL": "https://wiki.archlinux.org/index.php/Linux-ck",
			"URLPath": "/cgit/aur.git/snapshot/linux-ck.tar.gz",
			"Version": "5.16.12-1"
		}
	],
	"type": "search",
	"version": 5
}
`

type mockDB struct {
	db.Executor
}

func (m *mockDB) LocalPackage(string) alpm.IPackage {
	return nil
}

func (m *mockDB) PackageGroups(pkg alpm.IPackage) []string {
	return []string{}
}

func (m *mockDB) SyncPackages(...string) []alpm.IPackage {
	mockDB := mock.NewDB("core")
	linuxRepo := &mock.Package{
		PName:        "linux",
		PVersion:     "5.16.0",
		PDescription: "The Linux kernel and modules",
		PSize:        1,
		PISize:       1,
		PDB:          mockDB,
	}

	linuxZen := &mock.Package{
		PName:        "linux-zen",
		PVersion:     "5.16.0",
		PDescription: "The Linux ZEN kernel and modules",
		PSize:        1,
		PISize:       1,
		PDB:          mockDB,
	}

	return []alpm.IPackage{linuxRepo, linuxZen}
}

type mockDoer struct{}

func (m *mockDoer) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(validPayload)),
	}, nil
}

func TestSourceQueryBuilder(t *testing.T) {
	t.Parallel()
	type testCase struct {
		desc     string
		bottomUp bool
		want     string
	}

	testCases := []testCase{
		{desc: "bottomup", bottomUp: true, want: "\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\n    The Linux-ck kernel and modules with ck's hrtimer patches\n\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux-zen\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux ZEN kernel and modules\n\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux kernel and modules\n"},
		{
			desc: "topdown", bottomUp: false,
			want: "\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux kernel and modules\n\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux-zen\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux ZEN kernel and modules\n\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\n    The Linux-ck kernel and modules with ck's hrtimer patches\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			client, err := rpc.NewClient(rpc.WithHTTPClient(&mockDoer{}))
			require.NoError(t, err)

			queryBuilder := NewSourceQueryBuilder(client, client,
				text.NewLogger(io.Discard, bytes.NewBufferString(""), false, "test"),
				"votes", parser.ModeAny, "", tc.bottomUp, false, false)
			search := []string{"linux"}
			mockStore := &mockDB{}

			queryBuilder.Execute(context.Background(), mockStore, search)
			assert.Len(t, queryBuilder.aurQuery, 1)
			assert.Len(t, queryBuilder.repoQuery, 2)
			assert.Equal(t, 3, queryBuilder.Len())
			assert.Equal(t, "linux-ck", queryBuilder.aurQuery[0].Name)

			if tc.bottomUp {
				assert.Equal(t, "linux-zen", queryBuilder.repoQuery[0].Name())
				assert.Equal(t, "linux", queryBuilder.repoQuery[1].Name())
			} else {
				assert.Equal(t, "linux-zen", queryBuilder.repoQuery[1].Name())
				assert.Equal(t, "linux", queryBuilder.repoQuery[0].Name())
			}

			w := &strings.Builder{}
			queryBuilder.Results(w, mockStore, Detailed)

			wString := w.String()
			require.GreaterOrEqual(t, len(wString), 1)
			assert.Equal(t, tc.want, wString)
		})
	}
}
