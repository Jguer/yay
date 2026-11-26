package workdir

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	gosrc "github.com/Morganamilo/go-srcinfo"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v13/pkg/settings/exe"
	"github.com/Jguer/yay/v13/pkg/text"
)

// gitSource returns the clone directory name and remote URL makepkg uses for
// a git source entry, following get_filename and download_git in makepkg.
func gitSource(source string) (name, url string, ok bool) {
	url = source
	if before, after, found := strings.Cut(source, "::"); found {
		name, url = before, after
	}

	proto, _, found := strings.Cut(url, "://")
	if !found {
		return "", "", false
	}

	if proto, _, _ = strings.Cut(proto, "+"); proto != "git" {
		return "", "", false
	}

	url = strings.TrimPrefix(url, "git+")
	url, _, _ = strings.Cut(url, "#")
	url, _, _ = strings.Cut(url, "?")

	if name == "" {
		name = strings.TrimSuffix(url, "/")
		name = name[strings.LastIndex(name, "/")+1:]
		name, _, _ = strings.Cut(name, ".git")
	}

	return name, url, name != ""
}

// removeMovedGitClones deletes git clones left by an earlier build whose remote
// no longer matches the PKGBUILD source. makepkg refuses to reuse them
// ("is not a clone of"), so without this a moved upstream needs a manual
// cache cleanup. Deleting the clone lets makepkg clone the new URL.
func removeMovedGitClones(ctx context.Context, cmdBuilder exe.ICmdBuilder,
	logger *text.Logger, pkgBuildDir string,
) {
	srcinfo, err := gosrc.ParseFile(filepath.Join(pkgBuildDir, ".SRCINFO"))
	if err != nil {
		return
	}

	for _, source := range srcinfo.Source {
		name, url, ok := gitSource(source.Value)
		if !ok {
			continue
		}

		cloneDir := filepath.Join(pkgBuildDir, name)
		if info, err := os.Stat(cloneDir); err != nil || !info.IsDir() {
			continue
		}

		remoteURL, _, err := cmdBuilder.Capture(cmdBuilder.BuildGitCmd(ctx, cloneDir,
			"-c", "safe.bareRepository=all", "config", "--get", "remote.origin.url"))
		if err != nil {
			continue
		}

		remoteURL = strings.TrimSpace(remoteURL)
		if strings.TrimSuffix(remoteURL, ".git") == strings.TrimSuffix(url, ".git") {
			continue
		}

		logger.Warnln(gotext.Get("source moved from %s to %s, removing old clone: %s",
			remoteURL, url, text.Cyan(cloneDir)))

		if err := os.RemoveAll(cloneDir); err != nil {
			logger.Errorln(err)
		}
	}
}
