//go:build !integration
// +build !integration

package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Jguer/aur"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v12/pkg/db"
	"github.com/Jguer/yay/v12/pkg/db/mock"
	"github.com/Jguer/yay/v12/pkg/dep"
	mockaur "github.com/Jguer/yay/v12/pkg/dep/mock"
	"github.com/Jguer/yay/v12/pkg/text"
)

func TestGraphPackageRequiresSingleTarget(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	grapher := dep.NewGrapher(&mock.DBExecutor{}, &mockaur.MockAUR{}, false, false, false, false, false, logger)
	err := graphPackage(context.Background(), grapher, []string{"one", "two"})
	require.Error(t, err)
}

func TestGraphPackage(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	grapher := dep.NewGrapher(&mock.DBExecutor{
		LocalPackageFn: func(string) db.IPackage { return nil },
	}, &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{
				{
					Name:        "target",
					PackageBase: "target",
					Version:     "1.2.3",
				},
			}, nil
		},
	}, false, false, false, false, false, logger)

	output := captureStdout(t, func() {
		err := graphPackage(context.Background(), grapher, []string{"target"})
		require.NoError(t, err)
	})

	require.Contains(t, output, "digraph {")
	require.Contains(t, output, "layers map")
}

func captureStdout(t *testing.T, fn func()) string {
	orig := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = w
	defer func() {
		os.Stdout = orig
	}()

	buffer := &bytes.Buffer{}
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(buffer, r)
		close(done)
	}()

	fn()

	require.NoError(t, w.Close())
	<-done

	return buffer.String()
}
