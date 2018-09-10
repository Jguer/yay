package pacmanconf

type Repository struct {
	Name     string
	Servers  []string
	SigLevel []string
	Usage    []string
}

type Config struct {
	RootDir                string
	DBPath                 string
	CacheDir               []string
	HookDir                []string
	GPGDir                 string
	LogFile                string
	HoldPkg                []string
	IgnorePkg              []string
	IgnoreGroup            []string
	Architecture           string
	XferCommand            string
	NoUpgrade              []string
	NoExtract              []string
	CleanMethod            []string
	SigLevel               []string
	LocalFileSigLevel      []string
	RemoteFileSigLevel     []string
	UseSyslog              bool
	Color                  bool
	UseDelta               float64
	TotalDownload          bool
	CheckSpace             bool
	VerbosePkgLists        bool
	DisableDownloadTimeout bool
	Repos                  []Repository
}

func (conf *Config) Repository(name string) *Repository {
	for _, repo := range conf.Repos {
		if repo.Name == name {
			return &repo
		}
	}

	return nil
}
