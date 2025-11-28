//go:build !integration
// +build !integration

package completion

import (
	"bytes"
	"compress/gzip"
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
	returnBody       []byte
	returnStatusCode int
	returnErr        error
	wantURL          string
}

func (m *mockDoer) Get(url string) (*http.Response, error) {
	assert.Equal(m.t, m.wantURL, url)
	return &http.Response{
		StatusCode: m.returnStatusCode,
		Body:       io.NopCloser(bytes.NewReader(m.returnBody)),
	}, m.returnErr
}

func gzipString(s string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write([]byte(s))
	gz.Close()
	return buf.Bytes()
}

func Test_createAURList(t *testing.T) {
	t.Parallel()
	doer := &mockDoer{
		t:                t,
		wantURL:          "https://aur.archlinux.org/packages.gz",
		returnStatusCode: 200,
		returnBody:       []byte(samplePackageResp),
		returnErr:        nil,
	}
	out := &bytes.Buffer{}
	err := createAURList(context.Background(), doer, "https://aur.archlinux.org", out, nil)
	assert.NoError(t, err)
	gotOut := out.String()
	assert.Equal(t, expectPackageCompletion, gotOut)
}

func Test_createAURListGzip(t *testing.T) {
	t.Parallel()
	doer := &mockDoer{
		t:                t,
		wantURL:          "https://aur.archlinux.org/packages.gz",
		returnStatusCode: 200,
		returnBody:       gzipString(samplePackageResp),
		returnErr:        nil,
	}
	out := &bytes.Buffer{}
	err := createAURList(context.Background(), doer, "https://aur.archlinux.org", out, nil)
	assert.NoError(t, err)
	gotOut := out.String()
	assert.Equal(t, expectPackageCompletion, gotOut)
}

func Test_createAURListHTTPError(t *testing.T) {
	t.Parallel()
	doer := &mockDoer{
		t:                t,
		wantURL:          "https://aur.archlinux.org/packages.gz",
		returnStatusCode: 200,
		returnBody:       []byte(samplePackageResp),
		returnErr:        errors.New("Not available"),
	}

	out := &bytes.Buffer{}
	err := createAURList(context.Background(), doer, "https://aur.archlinux.org", out, nil)
	assert.EqualError(t, err, "Not available")
}

func Test_createAURListStatusError(t *testing.T) {
	t.Parallel()
	doer := &mockDoer{
		t:                t,
		wantURL:          "https://aur.archlinux.org/packages.gz",
		returnStatusCode: 503,
		returnBody:       []byte(samplePackageResp),
		returnErr:        nil,
	}

	out := &bytes.Buffer{}
	err := createAURList(context.Background(), doer, "https://aur.archlinux.org", out, nil)
	assert.EqualError(t, err, "invalid status code: 503")
}
