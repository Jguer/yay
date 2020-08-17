package settings

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Morganamilo/go-pacmanconf"
	"github.com/leonelquinteros/gotext"
	"github.com/pkg/errors"

	"github.com/Jguer/yay/v10/pkg/text"
)

type TargetMode int

// configFileName holds the name of the config file.
const configFileName string = "config.json"

// vcsFileName holds the name of the vcs file.
const vcsFileName string = "vcs.json"

const completionFileName string = "completion.cache"

const (
	ModeAny TargetMode = iota
	ModeAUR
	ModeRepo
)

type Runner interface {
	Capture(cmd *exec.Cmd, timeout int64) (stdout string, stderr string, err error)
	Show(cmd *exec.Cmd) error
}

type OSRunner struct {
}

func (r *OSRunner) Show(cmd *exec.Cmd) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("")
	}
	return nil
}

func (r *OSRunner) Capture(cmd *exec.Cmd, timeout int64) (stdout, stderr string, err error) {
	var outbuf, errbuf bytes.Buffer
	var timer *time.Timer
	timedOut := false

	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf
	err = cmd.Start()
	if err != nil {
		return "", "", err
	}

	if timeout != 0 {
		timer = time.AfterFunc(time.Duration(timeout)*time.Second, func() {
			err = cmd.Process.Kill()
			if err != nil {
				text.Errorln(err)
			}
			timedOut = true
		})
	}

	err = cmd.Wait()
	if timeout != 0 {
		timer.Stop()
	}
	if err != nil {
		return "", "", err
	}

	stdout = strings.TrimSpace(outbuf.String())
	stderr = strings.TrimSpace(errbuf.String())
	if timedOut {
		err = fmt.Errorf("command timed out")
	}

	return stdout, stderr, err
}

type Runtime struct {
	Mode           TargetMode
	SaveConfig     bool
	CompletionPath string
	ConfigPath     string
	VCSPath        string
	PacmanConf     *pacmanconf.Config
	CmdRunner      Runner
}

func MakeRuntime() (*Runtime, error) {
	cacheHome := ""
	configHome := ""

	runtime := &Runtime{
		Mode:           ModeAny,
		SaveConfig:     false,
		CompletionPath: "",
		CmdRunner:      &OSRunner{},
	}

	if configHome = os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		configHome = filepath.Join(configHome, "yay")
	} else if configHome = os.Getenv("HOME"); configHome != "" {
		configHome = filepath.Join(configHome, ".config", "yay")
	} else {
		return nil, errors.New(gotext.Get("%s and %s unset", "XDG_CONFIG_HOME", "HOME"))
	}

	if err := initDir(configHome); err != nil {
		return nil, err
	}

	if cacheHome = os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		cacheHome = filepath.Join(cacheHome, "yay")
	} else if cacheHome = os.Getenv("HOME"); cacheHome != "" {
		cacheHome = filepath.Join(cacheHome, ".cache", "yay")
	} else {
		return nil, errors.New(gotext.Get("%s and %s unset", "XDG_CACHE_HOME", "HOME"))
	}

	if err := initDir(cacheHome); err != nil {
		return runtime, err
	}

	runtime.ConfigPath = filepath.Join(configHome, configFileName)
	runtime.VCSPath = filepath.Join(cacheHome, vcsFileName)
	runtime.CompletionPath = filepath.Join(cacheHome, completionFileName)

	return runtime, nil
}

func initDir(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0o755); err != nil {
			return errors.New(gotext.Get("failed to create config directory '%s': %s", dir, err))
		}
	} else if err != nil {
		return err
	}

	return nil
}
