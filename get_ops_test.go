//go:build !integration
// +build !integration

package main

import (
	"context"
	"io"
	"net/http"
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
	t.Parallel()

	tests := []struct {
		name    string
		targets []string
		results []aur.Pkg
		wantErr bool
	}{
		{
			name:    "prints pkgbuild when package exists",
			targets: []string{"aur/pkg"},
			results: []aur.Pkg{{Name: "pkg", PackageBase: "pkg"}},
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
			gock.New("https://aur.archlinux.org").
				Get("/cgit/aur.git/plain/PKGBUILD").
				Reply(200).
				BodyString("pkgbuild")

			err := printPkgbuilds(&mockDBSearcher{}, &mockaur.MockAUR{
				GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
					return tc.results, nil
				},
			}, &http.Client{}, text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test"),
				tc.targets, parser.ModeAny, "https://aur.archlinux.org")
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

type mockDBSearcher struct{}

func (m *mockDBSearcher) SyncPackage(string) db.IPackage {
	return nil
}

func (m *mockDBSearcher) SyncPackageFromDB(string, string) db.IPackage {
	return nil
}
