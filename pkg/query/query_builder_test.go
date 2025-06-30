//go:build !integration
// +build !integration

package query

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/Jguer/aur"

	"github.com/Jguer/yay/v12/pkg/db/mock"
	mockaur "github.com/Jguer/yay/v12/pkg/dep/mock"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"

	"github.com/stretchr/testify/assert"
)

func TestSourceQueryBuilder(t *testing.T) {
	t.Parallel()
	type testCase struct {
		desc              string
		search            []string
		bottomUp          bool
		separateSources   bool
		sortBy            string
		verbosity         SearchVerbosity
		targetMode        parser.TargetMode
		singleLineResults bool
		searchBy          string
		showPackageURLs   bool
		wantResults       []string
		wantOutput        []string
	}

	testCases := []testCase{
		{
			desc:            "sort-by-votes bottomup separatesources",
			search:          []string{"linux"},
			bottomUp:        true,
			separateSources: true,
			sortBy:          "votes",
			verbosity:       Detailed,
			wantResults:     []string{"linux-ck", "linux-zen", "linux"},
			wantOutput: []string{
				"\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\n    The Linux-ck kernel and modules with ck's hrtimer patches\n",
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux-zen\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux ZEN kernel and modules\n",
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux kernel and modules\n",
			},
		},
		{
			desc:            "sort-by-votes topdown separatesources",
			search:          []string{"linux"},
			bottomUp:        false,
			separateSources: true,
			sortBy:          "votes",
			verbosity:       Detailed,
			wantResults:     []string{"linux", "linux-zen", "linux-ck"},
			wantOutput: []string{
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux kernel and modules\n",
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux-zen\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux ZEN kernel and modules\n",
				"\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\n    The Linux-ck kernel and modules with ck's hrtimer patches\n",
			},
		},
		{
			desc:            "sort-by-votes bottomup noseparatesources",
			search:          []string{"linux"},
			bottomUp:        true,
			separateSources: false,
			sortBy:          "votes",
			verbosity:       Detailed,
			wantResults:     []string{"linux-zen", "linux-ck", "linux"},
			wantOutput: []string{
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux-zen\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux ZEN kernel and modules\n",
				"\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\n    The Linux-ck kernel and modules with ck's hrtimer patches\n",
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux kernel and modules\n",
			},
		},
		{
			desc:            "sort-by-votes topdown noseparatesources",
			search:          []string{"linux"},
			bottomUp:        false,
			separateSources: false,
			sortBy:          "votes",
			verbosity:       Detailed,
			wantResults:     []string{"linux", "linux-ck", "linux-zen"},
			wantOutput: []string{
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux kernel and modules\n",
				"\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\n    The Linux-ck kernel and modules with ck's hrtimer patches\n",
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux-zen\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux ZEN kernel and modules\n",
			},
		},
		{
			desc:            "sort-by-name bottomup separatesources",
			search:          []string{"linux"},
			bottomUp:        true,
			separateSources: true,
			sortBy:          "name",
			verbosity:       Detailed,
			wantResults:     []string{"linux-ck", "linux", "linux-zen"},
			wantOutput: []string{
				"\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\n    The Linux-ck kernel and modules with ck's hrtimer patches\n",
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux kernel and modules\n",
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux-zen\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux ZEN kernel and modules\n",
			},
		},
		{
			desc:            "sort-by-name topdown separatesources",
			search:          []string{"linux"},
			bottomUp:        false,
			separateSources: true,
			sortBy:          "name",
			verbosity:       Detailed,
			wantResults:     []string{"linux-zen", "linux", "linux-ck"},
			wantOutput: []string{
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux-zen\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux ZEN kernel and modules\n",
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux kernel and modules\n",
				"\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\n    The Linux-ck kernel and modules with ck's hrtimer patches\n",
			},
		},
		{
			desc:            "sort-by-name bottomup noseparatesources",
			search:          []string{"linux"},
			bottomUp:        true,
			separateSources: false,
			sortBy:          "name",
			verbosity:       Detailed,
			wantResults:     []string{"linux", "linux-ck", "linux-zen"},
			wantOutput: []string{
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux kernel and modules\n",
				"\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\n    The Linux-ck kernel and modules with ck's hrtimer patches\n",
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux-zen\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux ZEN kernel and modules\n",
			},
		},
		{
			desc:            "sort-by-name topdown noseparatesources",
			search:          []string{"linux"},
			bottomUp:        false,
			separateSources: false,
			sortBy:          "name",
			verbosity:       Detailed,
			wantResults:     []string{"linux-zen", "linux-ck", "linux"},
			wantOutput: []string{
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux-zen\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux ZEN kernel and modules\n",
				"\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\n    The Linux-ck kernel and modules with ck's hrtimer patches\n",
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux kernel and modules\n",
			},
		},
		{
			desc:            "sort-by-votes bottomup separatesources number-menu",
			search:          []string{"linux"},
			bottomUp:        true,
			separateSources: true,
			sortBy:          "votes",
			verbosity:       NumberMenu,
			wantResults:     []string{"linux-ck", "linux-zen", "linux"},
			wantOutput: []string{
				"\x1b[35m3\x1b[0m \x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\n    The Linux-ck kernel and modules with ck's hrtimer patches\n",
				"\x1b[35m2\x1b[0m \x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux-zen\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux ZEN kernel and modules\n",
				"\x1b[35m1\x1b[0m \x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux kernel and modules\n",
			},
		},
		{
			desc:            "sort-by-votes topdown separatesources number-menu",
			search:          []string{"linux"},
			bottomUp:        false,
			separateSources: true,
			sortBy:          "votes",
			verbosity:       NumberMenu,
			wantResults:     []string{"linux", "linux-zen", "linux-ck"},
			wantOutput: []string{
				"\x1b[35m1\x1b[0m \x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux kernel and modules\n",
				"\x1b[35m2\x1b[0m \x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux-zen\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux ZEN kernel and modules\n",
				"\x1b[35m3\x1b[0m \x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\n    The Linux-ck kernel and modules with ck's hrtimer patches\n",
			},
		},
		{
			desc:            "sort-by-name bottomup separatesources number-menu",
			search:          []string{"linux"},
			bottomUp:        true,
			separateSources: true,
			sortBy:          "name",
			verbosity:       NumberMenu,
			wantResults:     []string{"linux-ck", "linux", "linux-zen"},
			wantOutput: []string{
				"\x1b[35m3\x1b[0m \x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\n    The Linux-ck kernel and modules with ck's hrtimer patches\n",
				"\x1b[35m2\x1b[0m \x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux kernel and modules\n",
				"\x1b[35m1\x1b[0m \x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux-zen\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux ZEN kernel and modules\n",
			},
		},
		{
			desc:            "sort-by-name topdown separatesources number-menu",
			search:          []string{"linux"},
			bottomUp:        false,
			separateSources: true,
			sortBy:          "name",
			verbosity:       NumberMenu,
			wantResults:     []string{"linux-zen", "linux", "linux-ck"},
			wantOutput: []string{
				"\x1b[35m1\x1b[0m \x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux-zen\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux ZEN kernel and modules\n",
				"\x1b[35m2\x1b[0m \x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux kernel and modules\n",
				"\x1b[35m3\x1b[0m \x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\n    The Linux-ck kernel and modules with ck's hrtimer patches\n",
			},
		},
		{
			desc:            "sort-by-name bottomup noseparatesources minimal",
			search:          []string{"linux"},
			bottomUp:        true,
			separateSources: false,
			sortBy:          "name",
			verbosity:       Minimal,
			wantResults:     []string{"linux", "linux-ck", "linux-zen"},
			wantOutput: []string{
				"linux\n",
				"linux-ck\n",
				"linux-zen\n",
			},
		},
		{
			desc:            "only-aur minimal",
			search:          []string{"linux"},
			bottomUp:        true,
			separateSources: true,
			sortBy:          "name",
			verbosity:       Minimal,
			targetMode:      parser.ModeAUR,
			wantResults:     []string{"linux-ck"},
			wantOutput: []string{
				"linux-ck\n",
			},
		},
		{
			desc:            "only-repo minimal",
			search:          []string{"linux"},
			bottomUp:        true,
			separateSources: true,
			sortBy:          "name",
			verbosity:       Minimal,
			targetMode:      parser.ModeRepo,
			wantResults:     []string{"linux", "linux-zen"},
			wantOutput: []string{
				"linux\n",
				"linux-zen\n",
			},
		},
		{
			desc:              "sort-by-name singleline",
			search:            []string{"linux"},
			bottomUp:          true,
			separateSources:   true,
			sortBy:            "name",
			verbosity:         Detailed,
			singleLineResults: true,
			wantResults:       []string{"linux-ck", "linux", "linux-zen"},
			wantOutput: []string{
				"\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\tThe Linux-ck kernel and modules with ck's hrtimer patches\n",
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\tThe Linux kernel and modules\n",
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux-zen\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\tThe Linux ZEN kernel and modules\n",
			},
		},
		{
			desc:            "sort-by-name showpackageurls",
			search:          []string{"linux"},
			bottomUp:        true,
			separateSources: true,
			sortBy:          "name",
			verbosity:       Detailed,
			showPackageURLs: true,
			wantResults:     []string{"linux-ck", "linux", "linux-zen"},
			wantOutput: []string{
				"\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\n    The Linux-ck kernel and modules with ck's hrtimer patches\n    Package URL: https://aur.archlinux.org/packages/linux-ck\n",
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux kernel and modules\n    Package URL: https://archlinux.org/packages/core/any/linux\n",
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux-zen\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\n    The Linux ZEN kernel and modules\n    Package URL: https://archlinux.org/packages/core/any/linux-zen\n",
			},
		},
		{
			desc:              "sort-by-name singleline showpackageurls",
			search:            []string{"linux"},
			bottomUp:          true,
			separateSources:   true,
			sortBy:            "name",
			verbosity:         Detailed,
			singleLineResults: true,
			showPackageURLs:   true,
			wantResults:       []string{"linux-ck", "linux", "linux-zen"},
			wantOutput: []string{
				"\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\tThe Linux-ck kernel and modules with ck's hrtimer patches\tPackage URL: https://aur.archlinux.org/packages/linux-ck\n",
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\tThe Linux kernel and modules\tPackage URL: https://archlinux.org/packages/core/any/linux\n",
				"\x1b[1m\x1b[33mcore\x1b[0m\x1b[0m/\x1b[1mlinux-zen\x1b[0m \x1b[36m5.16.0\x1b[0m\x1b[1m (1.0 B 1.0 B) \x1b[0m\tThe Linux ZEN kernel and modules\tPackage URL: https://archlinux.org/packages/core/any/linux-zen\n",
			},
		},
		{
			desc:            "sort-by-name search-by-name",
			search:          []string{"linux-ck"},
			bottomUp:        true,
			separateSources: true,
			sortBy:          "name",
			verbosity:       Detailed,
			searchBy:        "name",
			targetMode:      parser.ModeAUR,
			wantResults:     []string{"linux-ck"},
			wantOutput: []string{
				"\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\n    The Linux-ck kernel and modules with ck's hrtimer patches\n",
			},
		},
		{
			desc:            "only-aur search-by-several-terms",
			search:          []string{"linux-ck", "hrtimer"},
			bottomUp:        true,
			separateSources: true,
			verbosity:       Detailed,
			targetMode:      parser.ModeAUR,
			wantResults:     []string{"linux-ck"},
			wantOutput: []string{
				"\x1b[1m\x1b[34maur\x1b[0m\x1b[0m/\x1b[1mlinux-ck\x1b[0m \x1b[36m5.16.12-1\x1b[0m\x1b[1m (+450\x1b[0m \x1b[1m1.51) \x1b[0m\n    The Linux-ck kernel and modules with ck's hrtimer patches\n",
			},
		},
	}

	mockDB := &mock.DBExecutor{
		SyncPackagesFn: func(pkgs ...string) []mock.IPackage {
			mockDB := mock.NewDB("core")
			return []mock.IPackage{
				&mock.Package{
					PName:         "linux",
					PVersion:      "5.16.0",
					PDescription:  "The Linux kernel and modules",
					PArchitecture: "any",
					PSize:         1,
					PISize:        1,
					PDB:           mockDB,
				},
				&mock.Package{
					PName:         "linux-zen",
					PVersion:      "5.16.0",
					PDescription:  "The Linux ZEN kernel and modules",
					PArchitecture: "any",
					PSize:         1,
					PISize:        1,
					PDB:           mockDB,
				},
			}
		},
		LocalPackageFn: func(string) mock.IPackage {
			return nil
		},
	}

	mockAUR := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{
				{
					Description:    "The Linux-ck kernel and modules with ck's hrtimer patches",
					FirstSubmitted: 1311346274,
					ID:             1045311,
					LastModified:   1646250901,
					Maintainer:     "graysky",
					Name:           "linux-ck",
					NumVotes:       450,
					OutOfDate:      0,
					PackageBase:    "linux-ck",
					PackageBaseID:  50911,
					Popularity:     1.511141,
					URL:            "https://wiki.archlinux.org/index.php/Linux-ck",
					URLPath:        "/cgit/aur.git/snapshot/linux-ck.tar.gz",
					Version:        "5.16.12-1",
				},
			}, nil
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			w := &strings.Builder{}
			queryBuilder := NewSourceQueryBuilder(mockAUR,
				text.NewLogger(w, io.Discard, strings.NewReader(""), false, "test"),
				tc.sortBy, tc.targetMode, tc.searchBy, tc.bottomUp,
				tc.singleLineResults, tc.separateSources, tc.showPackageURLs)

			queryBuilder.Execute(context.Background(), mockDB, tc.search)

			assert.Len(t, queryBuilder.results, len(tc.wantResults))
			assert.Equal(t, len(tc.wantResults), queryBuilder.Len())
			for i, name := range tc.wantResults {
				assert.Equal(t, name, queryBuilder.results[i].name)
			}

			queryBuilder.Results(mockDB, tc.verbosity)

			assert.Equal(t, strings.Join(tc.wantOutput, ""), w.String())
		})
	}
}
