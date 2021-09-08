package download

import (
	"context"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Jguer/go-alpm/v2"

	"github.com/Jguer/yay/v11/pkg/db"
	"github.com/Jguer/yay/v11/pkg/settings/exe"
)

type testRunner struct{}

func (t *testRunner) Capture(cmd *exec.Cmd) (stdout string, stderr string, err error) {
	return "", "", nil
}

func (t *testRunner) Show(cmd *exec.Cmd) error {
	return nil
}

type testGitBuilder struct {
	index         int
	test          *testing.T
	want          string
	parentBuilder *exe.CmdBuilder
}

func (t *testGitBuilder) BuildGitCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd {
	cmd := t.parentBuilder.BuildGitCmd(ctx, dir, extraArgs...)

	if t.want != "" {
		assert.Equal(t.test, t.want, cmd.String())
	}

	return cmd
}

func (c *testGitBuilder) Show(cmd *exec.Cmd) error {
	return c.parentBuilder.Show(cmd)
}

func (c *testGitBuilder) Capture(cmd *exec.Cmd) (stdout, stderr string, err error) {
	return c.parentBuilder.Capture(cmd)
}

type (
	testDB struct {
		alpm.IDB
		name string
	}
	testPackage struct {
		db.IPackage
		name string
		base string
		db   *testDB
	}
	testDBSearcher struct {
		absPackagesDB map[string]string
	}

	testClient struct {
		t       *testing.T
		wantURL string
		body    string
		status  int
	}
)

func (d *testDB) Name() string {
	return d.name
}

func (p *testPackage) Name() string {
	return p.name
}

func (p *testPackage) Base() string {
	return p.base
}

func (p *testPackage) DB() alpm.IDB {
	return p.db
}

func (d *testDBSearcher) SyncPackage(name string) db.IPackage {
	if v, ok := d.absPackagesDB[name]; ok {
		return &testPackage{
			name: name,
			base: name,
			db:   &testDB{name: v},
		}
	}

	return nil
}

func (d *testDBSearcher) SatisfierFromDB(name string, db string) db.IPackage {
	if v, ok := d.absPackagesDB[name]; ok && v == db {
		return &testPackage{
			name: name,
			base: name,
			db:   &testDB{name: v},
		}
	}

	return nil
}

func (t *testClient) Get(url string) (*http.Response, error) {
	assert.Equal(t.t, t.wantURL, url)
	return &http.Response{StatusCode: t.status, Body: io.NopCloser(strings.NewReader(t.body))}, nil
}
