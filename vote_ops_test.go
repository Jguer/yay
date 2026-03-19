//go:build !integration
// +build !integration

package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/Jguer/aur"
	"github.com/stretchr/testify/require"

	mockaur "github.com/Jguer/yay/v12/pkg/dep/mock"
	"github.com/Jguer/yay/v12/pkg/text"
)

func TestHandlePackageVoteNoResults(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")
	err := handlePackageVote(context.Background(), []string{"missing"}, &mockaur.MockAUR{
		GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return nil, nil
		},
	}, logger, nil, true)
	require.NoError(t, err)
}
