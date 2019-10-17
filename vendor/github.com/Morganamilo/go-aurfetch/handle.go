package aurfetch

type Handle struct {
	AURURL         string
	CacheDir       string
	PatchDir       string
	GitCommand     string
	GitArgs        []string
	GitCommandArgs []string
	GitEnvironment []string
}

func MakeHandle(cacheDir string, patchDir string) Handle {
	handle := Handle{
		AURURL:         "https://aur.archlinux.org",
		CacheDir:       cacheDir,
		PatchDir:       patchDir,
		GitCommand:     "git",
		GitArgs:        []string{},
		GitCommandArgs: []string{},
		GitEnvironment: []string{},
	}

	return handle
}
