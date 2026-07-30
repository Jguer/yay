//go:build !integration

package pkgbuildrepo

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// GIVEN repo URLs in various makepkg-source forms
// WHEN resolved against a cache dir
// THEN git URLs clone into the cache while file:// and bare paths are used in
// place.
func TestResolveLocation(t *testing.T) {
	t.Parallel()

	cache := "/cache"

	tests := []struct {
		name string
		url  string
		want Location
	}{
		{
			name: "remote-https",
			url:  "https://github.com/Jguer/yay-PKGBUILD",
			want: Location{Dir: filepath.Join(cache, "remote-https"), IsGit: true, CloneURL: "https://github.com/Jguer/yay-PKGBUILD"},
		},
		{
			name: "ssh-dot-git",
			url:  "ssh://git@example.invalid/repo.git",
			want: Location{Dir: filepath.Join(cache, "ssh-dot-git"), IsGit: true, CloneURL: "ssh://git@example.invalid/repo.git"},
		},
		{
			name: "local-git",
			url:  "git+file:///srv/pkgbuild-repo",
			want: Location{Dir: filepath.Join(cache, "local-git"), IsGit: true, CloneURL: "file:///srv/pkgbuild-repo"},
		},
		{
			name: "local-dir",
			url:  "file:///srv/pkgbuilds",
			want: Location{Dir: "/srv/pkgbuilds", IsGit: false},
		},
		{
			name: "bare-path",
			url:  "/srv/local",
			want: Location{Dir: "/srv/local", IsGit: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, Resolve(tt.name, tt.url, cache))
		})
	}
}

func TestResolveDoesNotTreatInsecureGitURLsAsRemoteRepositories(t *testing.T) {
	t.Parallel()

	cache := "/cache"
	for _, url := range []string{"http://example.invalid/repo.git", "git://example.invalid/repo.git", "git+http://example.invalid/repo.git"} {
		assert.False(t, Resolve("repo", url, cache).IsGit, url)
	}
}
