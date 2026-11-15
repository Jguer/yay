//go:build !integration

package aursource

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/Jguer/aur"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mockaur "github.com/Jguer/yay/v13/pkg/dep/mock"
	"github.com/Jguer/yay/v13/pkg/settings"
	"github.com/Jguer/yay/v13/pkg/text"
)

func returning(pkgs []aur.Pkg, err error) aur.QueryClient {
	return &mockaur.MockAUR{GetFn: func(context.Context, *aur.Query) ([]aur.Pkg, error) {
		return pkgs, err
	}}
}

func testLogger() *text.Logger {
	return text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), false, "test")
}

var testSources = []settings.AURSource{
	{Name: "aur", URL: "https://aur.archlinux.org"},
	{Name: "mirror", URL: "https://aur.example.org"},
}

func TestGetMergesFirstSourceWins(t *testing.T) {
	t.Parallel()

	c := New(testSources, []aur.QueryClient{
		returning([]aur.Pkg{{Name: "shared", Version: "1"}, {Name: "only-aur", PackageBase: "only-aur"}}, nil),
		returning([]aur.Pkg{{Name: "shared", Version: "2"}, {Name: "only-mirror", PackageBase: "mirror-base"}}, nil),
	}, testLogger())

	pkgs, err := c.Get(context.Background(), &aur.Query{Needles: []string{"x"}, By: aur.Name})
	require.NoError(t, err)

	require.Len(t, pkgs, 3)
	assert.Equal(t, "shared", pkgs[0].Name)
	assert.Equal(t, "1", pkgs[0].Version, "earlier source must win name conflicts")
	assert.Equal(t, "only-aur", pkgs[1].Name)
	assert.Equal(t, "only-mirror", pkgs[2].Name)

	assert.Equal(t, "aur", c.SourceFor("shared").Name)
	assert.Equal(t, "mirror", c.SourceFor("only-mirror").Name)
	assert.Equal(t, "mirror", c.SourceFor("mirror-base").Name, "package base is recorded too")
	assert.Equal(t, "aur", c.SourceFor("never-seen").Name, "unknown packages default to the first source")

	assert.Equal(t, []string{"https://aur.example.org", "https://aur.archlinux.org"}, c.URLsFor("mirror-base"))
	assert.Equal(t, []string{"https://aur.archlinux.org", "https://aur.example.org"}, c.URLsFor("only-aur"))
}

func TestGetToleratesPartialFailure(t *testing.T) {
	t.Parallel()

	c := New(testSources, []aur.QueryClient{
		returning(nil, errors.New("aur down")),
		returning([]aur.Pkg{{Name: "pkg"}}, nil),
	}, testLogger())

	pkgs, err := c.Get(context.Background(), &aur.Query{})
	require.NoError(t, err)
	require.Len(t, pkgs, 1)
	assert.Equal(t, "mirror", c.SourceFor("pkg").Name)
}

func TestGetFailsWhenAllSourcesFail(t *testing.T) {
	t.Parallel()

	c := New(testSources, []aur.QueryClient{
		returning(nil, errors.New("aur down")),
		returning(nil, errors.New("mirror down")),
	}, testLogger())

	_, err := c.Get(context.Background(), &aur.Query{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "aur down")
	assert.ErrorContains(t, err, "mirror down")
}

func TestSingleSourcePassesThrough(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	c := New(testSources[:1], []aur.QueryClient{returning(nil, wantErr)}, testLogger())

	_, err := c.Get(context.Background(), &aur.Query{})
	require.ErrorIs(t, err, wantErr)
	assert.Equal(t, []string{"https://aur.archlinux.org"}, c.URLsFor("anything"))
}
