//go:build !integration
// +build !integration

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Jguer/aur"
	"github.com/stretchr/testify/require"
	gock "gopkg.in/h2non/gock.v1"

	"github.com/Jguer/yay/v12/pkg/db"
	mockaur "github.com/Jguer/yay/v12/pkg/dep/mock"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
)

func TestPrintPkgbuilds(t *testing.T) {
	tests := []struct {
		name          string
		targets       []string
		results       []aur.Pkg
		pkgbuildPager string
		wantOutput    string
		wantPager     string
		wantErr       bool
	}{
		{
			name:       "prints pkgbuild when package exists",
			targets:    []string{"aur/pkg"},
			results:    []aur.Pkg{{Name: "pkg", PackageBase: "pkg"}},
			wantOutput: "\n\n# aur/pkg\n\npkgbuild",
		},
		{
			name:      "pipes pkgbuild through configured pager",
			targets:   []string{"aur/pkg"},
			results:   []aur.Pkg{{Name: "pkg", PackageBase: "pkg"}},
			wantPager: "\n\n# aur/pkg\n\npkgbuild",
		},
		{
			name:    "returns error when package does not exist",
			targets: []string{"aur/found", "aur/missing"},
			results: nil,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			defer gock.Off()
			pagerOut := ""
			pkgbuildPager := tc.pkgbuildPager
			if tc.wantPager != "" {
				pagerOut = filepath.Join(t.TempDir(), "pager-out")
				pkgbuildPager = fmt.Sprintf("cat > %s", strconv.Quote(pagerOut))
			}

			gock.New("https://aur.archlinux.org").
				Get("/cgit/aur.git/plain/PKGBUILD").
				Reply(200).
				BodyString("pkgbuild")

			var stdout bytes.Buffer
			err := printPkgbuilds(context.Background(), &mockDBSearcher{}, &mockaur.MockAUR{
				GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
					return tc.results, nil
				},
			}, &http.Client{}, text.NewLogger(&stdout, io.Discard, strings.NewReader(""), true, "test"),
				tc.targets, parser.ModeAny, "https://aur.archlinux.org", pkgbuildPager)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantOutput, stdout.String())
			if tc.wantPager != "" {
				got, readErr := os.ReadFile(pagerOut)
				require.NoError(t, readErr)
				require.Equal(t, tc.wantPager, string(got))
			}
		})
	}
}

func TestPrintPkgbuildsReturnsPagerError(t *testing.T) {
	defer gock.Off()
	gock.New("https://aur.archlinux.org").
		Get("/cgit/aur.git/plain/PKGBUILD").
		Reply(200).
		BodyString("pkgbuild")

	err := printPkgbuilds(context.Background(), &mockDBSearcher{}, &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{Name: "pkg", PackageBase: "pkg"}}, nil
		},
	}, &http.Client{}, text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test"),
		[]string{"aur/pkg"}, parser.ModeAny, "https://aur.archlinux.org", "exit 42")

	require.ErrorContains(t, err, "failed to run pkgbuild pager")
}

type mockDBSearcher struct{}

func (m *mockDBSearcher) SyncPackage(string) db.IPackage {
	return nil
}

func (m *mockDBSearcher) SyncPackageFromDB(string, string) db.IPackage {
	return nil
}
