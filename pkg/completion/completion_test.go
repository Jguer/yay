package completion

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
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

func Test_createAURList(t *testing.T) {
	defer gock.Off()

	gock.New("https://aur.archlinux.org").
		Get("/packages.gz").
		Reply(200).
		BodyString(samplePackageResp)
	out := &bytes.Buffer{}
	err := createAURList("https://aur.archlinux.org", out)
	assert.NoError(t, err)
	gotOut := out.String()
	assert.Equal(t, expectPackageCompletion, gotOut)
}

func Test_createAURListHTTPError(t *testing.T) {
	defer gock.Off()

	gock.New("https://aur.archlinux.org").
		Get("/packages.gz").
		ReplyError(errors.New("Not available"))
	out := &bytes.Buffer{}
	err := createAURList("https://aur.archlinux.org", out)
	assert.EqualError(t, err, "Get \"https://aur.archlinux.org/packages.gz\": Not available")
}

func Test_createAURListStatusError(t *testing.T) {
	defer gock.Off()

	gock.New("https://aur.archlinux.org").
		Get("/packages.gz").
		Reply(503).
		BodyString(samplePackageResp)
	out := &bytes.Buffer{}
	err := createAURList("https://aur.archlinux.org", out)
	assert.EqualError(t, err, "invalid status code: 503")
}
