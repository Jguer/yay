package pacmanconf

import (
	"fmt"
	"github.com/Morganamilo/go-pacmanconf/ini"
	"strconv"
)

type callbackData struct {
	conf *Config
	repo *Repository
}

func parseCallback(fileName string, line int, section string,
	key string, value string, data interface{}) error {
	if line < 0 {
		return fmt.Errorf("unable to read file: %s: %s", fileName, section)
	}

	d, ok := data.(*callbackData)
	if !ok {
		return fmt.Errorf("type assert failed when parsing: %s", fileName)
	}

	if key == "" && value == "" {
		if section == "options" {
			d.repo = nil
		} else {
			d.conf.Repos = append(d.conf.Repos, Repository{})
			d.repo = &d.conf.Repos[len(d.conf.Repos)-1]
			d.repo.Name = section
		}

		return nil
	}

	if section == "" {
		return fmt.Errorf("line %d is not in a section: %s", line, fileName)
	}

	if d.repo == nil {
		setOption(d.conf, key, value)
	} else {
		setRepo(d.repo, key, value)
	}

	return nil
}

func setRepo(repo *Repository, key string, value string) {
	switch key {
	case "Server":
		repo.Servers = append(repo.Servers, value)
	case "SigLevel":
		repo.SigLevel = append(repo.SigLevel, value)
	case "Usage":
		repo.Usage = append(repo.Usage, value)
	}
}

func setOption(conf *Config, key string, value string) {
	switch key {
	case "RootDir":
		conf.RootDir = value
	case "DBPath":
		conf.DBPath = value
	case "CacheDir":
		conf.CacheDir = append(conf.CacheDir, value)
	case "HookDir":
		conf.HookDir = append(conf.HookDir, value)
	case "GPGDir":
		conf.GPGDir = value
	case "LogFile":
		conf.LogFile = value
	case "HoldPkg":
		conf.HoldPkg = append(conf.HoldPkg, value)
	case "IgnorePkg":
		conf.IgnorePkg = append(conf.IgnorePkg, value)
	case "IgnoreGroup":
		conf.IgnoreGroup = append(conf.IgnoreGroup, value)
	case "Architecture":
		conf.Architecture = value
	case "XferCommand":
		conf.XferCommand = value
	case "NoUpgrade":
		conf.NoUpgrade = append(conf.NoUpgrade, value)
	case "NoExtract":
		conf.NoExtract = append(conf.NoExtract, value)
	case "CleanMethod":
		conf.CleanMethod = append(conf.CleanMethod, value)
	case "SigLevel":
		conf.SigLevel = append(conf.SigLevel, value)
	case "LocalFileSigLevel":
		conf.LocalFileSigLevel = append(conf.LocalFileSigLevel, value)
	case "RemoteFileSigLevel":
		conf.RemoteFileSigLevel = append(conf.RemoteFileSigLevel, value)
	case "UseSyslog":
		conf.UseSyslog = true
	case "Color":
		conf.Color = true
	case "UseDelta":
		f, err := strconv.ParseFloat(value, 64)
		if err == nil {
			conf.UseDelta = f
		}
	case "TotalDownload":
		conf.TotalDownload = true
	case "CheckSpace":
		conf.CheckSpace = true
	case "VerbosePkgLists":
		conf.VerbosePkgLists = true
	case "DisableDownloadTimeout":
		conf.DisableDownloadTimeout = true
	}
}

func Parse(iniData string) (*Config, error) {
	data := callbackData{&Config{}, nil}
	err := ini.Parse(iniData, parseCallback, &data)
	return data.conf, err
}

func PacmanConf(args ...string) (*Config, string, error) {
	stdout, stderr, err := pacmanconf(args)

	if err != nil {
		return nil, stderr, err
	}

	conf, err := Parse(stdout)

	return conf, "", err
}

func ParseFile(path string) (*Config, string, error) {
	return PacmanConf("--config", path)
}
