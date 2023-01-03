package completion

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

const samplePackageResp = `
# AUR package list, generated on Fri, 24 Jul 2020 22:05:22 GMT
cytadela
bitefusion
globs-svn
ri-li
globs-benchmarks-svn
dunelegacy
lumina
eternallands-sound
`

const expectPackageCompletion = `cytadela	AUR
bitefusion	AUR
globs-svn	AUR
ri-li	AUR
globs-benchmarks-svn	AUR
dunelegacy	AUR
lumina	AUR
eternallands-sound	AUR
`

type mockDoer struct {
	t                *testing.T
	returnBody       string
	returnStatusCode int
	returnErr        error
	wantUrl          string
}

func (m *mockDoer) Do(req *http.Request) (*http.Response, error) {
	assert.Equal(m.t, m.wantUrl, req.URL.String())
	return &http.Response{
		StatusCode: m.returnStatusCode,
		Body:       io.NopCloser(bytes.NewBufferString(m.returnBody)),
	}, m.returnErr
}

func Test_createAURList(t *testing.T) {
	t.Parallel()
	doer := &mockDoer{
		t:                t,
		wantUrl:          "https://aur.archlinux.org/packages.gz",
		returnStatusCode: 200,
		returnBody:       samplePackageResp,
		returnErr:        nil,
	}
	out := &bytes.Buffer{}
	err := createAURList(context.Background(), doer, "https://aur.archlinux.org", out)
	assert.NoError(t, err)
	gotOut := out.String()
	assert.Equal(t, expectPackageCompletion, gotOut)
}

func Test_createAURListHTTPError(t *testing.T) {
	t.Parallel()
	doer := &mockDoer{
		t:                t,
		wantUrl:          "https://aur.archlinux.org/packages.gz",
		returnStatusCode: 200,
		returnBody:       samplePackageResp,
		returnErr:        errors.New("Not available"),
	}

	out := &bytes.Buffer{}
	err := createAURList(context.Background(), doer, "https://aur.archlinux.org", out)
	assert.EqualError(t, err, "Not available")
}

func Test_createAURListStatusError(t *testing.T) {
	t.Parallel()
	doer := &mockDoer{
		t:                t,
		wantUrl:          "https://aur.archlinux.org/packages.gz",
		returnStatusCode: 503,
		returnBody:       samplePackageResp,
		returnErr:        nil,
	}

	out := &bytes.Buffer{}
	err := createAURList(context.Background(), doer, "https://aur.archlinux.org", out)
	assert.EqualError(t, err, "invalid status code: 503")
}
