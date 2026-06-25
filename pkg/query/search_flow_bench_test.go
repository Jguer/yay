//go:build !integration

package query

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/Jguer/aur"

	"github.com/Jguer/yay/v13/pkg/db/mock"
	mockaur "github.com/Jguer/yay/v13/pkg/dep/mock"
	"github.com/Jguer/yay/v13/pkg/settings/parser"
	"github.com/Jguer/yay/v13/pkg/text"
)

func buildSearchFlowBenchmarkData(repoCount, aurCount int) (*mock.DBExecutor, *mockaur.MockAUR, *text.Logger, []string) {
	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "bench")
	extraDB := mock.NewDB("extra")
	communityDB := mock.NewDB("community")

	repoPkgs := make([]mock.IPackage, 0, repoCount)
	for i := range repoCount {
		db := extraDB
		if i%2 == 1 {
			db = communityDB
		}

		repoPkgs = append(repoPkgs, &mock.Package{
			PBase:         fmt.Sprintf("yay-tool-%04d", i),
			PName:         fmt.Sprintf("yay-tool-%04d", i),
			PVersion:      fmt.Sprintf("1.%d.0", i%17),
			PDescription:  fmt.Sprintf("Yet another yay tool package %04d for search benchmarking", i),
			PSize:         int64(1024 + i),
			PISize:        int64(2048 + i),
			PDB:           db,
			PArchitecture: "x86_64",
			PProvides:     mock.DependList{Depends: []mock.Depend{{Name: fmt.Sprintf("yay-provider-%04d", i)}, {Name: "yay"}}},
		})
	}

	aurPkgs := make([]aur.Pkg, 0, aurCount)
	for i := range aurCount {
		aurPkgs = append(aurPkgs, aur.Pkg{
			Description:    fmt.Sprintf("Yet another yay helper package %04d with search benchmark metadata", i),
			FirstSubmitted: 1_500_000_000 + i,
			ID:             2_000_000 + i,
			LastModified:   1_760_000_000 + i,
			Maintainer:     "bench",
			Name:           fmt.Sprintf("yay-aur-%04d", i),
			NumVotes:       100 + (i % 250),
			OutOfDate:      0,
			PackageBase:    fmt.Sprintf("yay-aur-%04d", i),
			PackageBaseID:  300_000 + i,
			Popularity:     1.0 + float64(i%100)/10,
			URL:            "https://example.invalid/yay",
			URLPath:        "/snapshot.tar.gz",
			Version:        fmt.Sprintf("2.%d.0", i%23),
			Provides:       []string{"yay"},
		})
	}

	mockDB := &mock.DBExecutor{
		ReposFn: func() []string {
			return []string{"extra", "community"}
		},
		SyncPackagesFn: func(pkgs ...string) []mock.IPackage {
			return repoPkgs
		},
		LocalPackageFn: func(string) mock.IPackage {
			return nil
		},
	}

	mockAUR := &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return aurPkgs, nil
		},
	}

	return mockDB, mockAUR, logger, []string{"yay"}
}

func BenchmarkExecuteSearchFlowLarge(b *testing.B) {
	mockDB, mockAUR, logger, searchTerms := buildSearchFlowBenchmarkData(1500, 1500)

	b.ReportAllocs()
	for b.Loop() {
		qb := NewSourceQueryBuilder(
			mockAUR,
			logger,
			"",
			parser.ModeAny,
			"",
			false,
			false,
			true,
		)
		qb.Execute(b.Context(), mockDB, searchTerms)
	}
}
