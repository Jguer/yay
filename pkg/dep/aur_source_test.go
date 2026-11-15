//go:build !integration

package dep

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/Jguer/aur"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Jguer/yay/v13/pkg/db/mock"
	mockaur "github.com/Jguer/yay/v13/pkg/dep/mock"
	"github.com/Jguer/yay/v13/pkg/text"
)

type pinningAUR struct {
	mockaur.MockAUR
	pinned map[string]string
}

func (p *pinningAUR) PinAURSource(pkgName, source string) bool {
	if source != "mirror" {
		return false
	}

	p.pinned[pkgName] = source

	return true
}

func (p *pinningAUR) UnpinAURSource(pkgName string) {
	p.pinned[pkgName] = "<unpinned>"
}

func TestGrapher_GraphFromTargetsAURSourcePrefix(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	dbExecutor := &mock.DBExecutor{
		LocalPackageFn: func(string) mock.IPackage { return nil },
		SatisfierFromDBFn: func(name, db string) (mock.IPackage, error) {
			t.Fatalf("mirror/%s must not be looked up as a pacman repository", name)
			return nil, nil
		},
	}

	client := &pinningAUR{
		MockAUR: mockaur.MockAUR{GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{Name: "foo", PackageBase: "foo", Version: "1"}}, nil
		}},
		pinned: map[string]string{},
	}

	grapher := NewGrapher(dbExecutor, client, false, false, false, false, false, logger)
	graph, err := grapher.GraphFromTargets(context.Background(), nil, []string{"mirror/foo"})
	require.NoError(t, err)
	require.True(t, graph.Exists("foo"))
	assert.Equal(t, AUR, graph.GetNodeInfo("foo").Value.Source)
	assert.Equal(t, map[string]string{"foo": "mirror"}, client.pinned)
}

func TestGrapher_GraphFromTargetsAURPrefixUnpins(t *testing.T) {
	t.Parallel()

	logger := text.NewLogger(io.Discard, io.Discard, strings.NewReader(""), true, "test")
	dbExecutor := &mock.DBExecutor{
		LocalPackageFn:  func(string) mock.IPackage { return nil },
		SyncSatisfierFn: func(string) mock.IPackage { return nil },
		PackagesFromGroupFn: func(string) []mock.IPackage {
			return nil
		},
	}

	client := &pinningAUR{
		MockAUR: mockaur.MockAUR{GetFn: func(ctx context.Context, query *aur.Query) ([]aur.Pkg, error) {
			return []aur.Pkg{{Name: "foo", PackageBase: "foo", Version: "1"}, {Name: "bar", PackageBase: "bar", Version: "1"}}, nil
		}},
		pinned: map[string]string{},
	}

	grapher := NewGrapher(dbExecutor, client, false, false, false, false, false, logger)
	graph, err := grapher.GraphFromTargets(context.Background(), nil, []string{"aur/foo", "bar"})
	require.NoError(t, err)
	require.True(t, graph.Exists("foo"))
	require.True(t, graph.Exists("bar"))
	assert.Equal(t, map[string]string{"foo": "<unpinned>"}, client.pinned,
		"aur/<pkg> drops a recorded source, a plain name keeps it")
}
