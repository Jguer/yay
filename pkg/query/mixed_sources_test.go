package query

import (
	"context"
	"strings"
	"testing"

	"github.com/Jguer/aur/rpc"

	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMixedSourceQueryBuilder(t *testing.T) {
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

			w := &strings.Builder{}
			queryBuilder := NewMixedSourceQueryBuilder(client,
				text.NewLogger(w, strings.NewReader(""), false, "test"),
				"votes", parser.ModeAny, "", tc.bottomUp, false)
			search := []string{"linux"}
			mockStore := &mockDB{}

			require.NoError(t, err)
			queryBuilder.Execute(context.Background(), mockStore, search)
			assert.Len(t, queryBuilder.results, 3)
			assert.Equal(t, 3, queryBuilder.Len())

			if tc.bottomUp {
				assert.Equal(t, "linux-ck", queryBuilder.results[0].name)
				assert.Equal(t, "linux-zen", queryBuilder.results[1].name)
				assert.Equal(t, "linux", queryBuilder.results[2].name)
			} else {
				assert.Equal(t, "linux-ck", queryBuilder.results[2].name)
				assert.Equal(t, "linux-zen", queryBuilder.results[1].name)
				assert.Equal(t, "linux", queryBuilder.results[0].name)
			}

			queryBuilder.Results(w, mockStore, Detailed)

			wString := w.String()
			require.GreaterOrEqual(t, len(wString), 1, wString)
			assert.Equal(t, tc.want, wString)
		})
	}
}
