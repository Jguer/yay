package exe

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/settings/parser"
	"github.com/Jguer/yay/v10/pkg/text"
)

const SudoLoopDuration = 241

type GitCmdBuilder interface {
	Runner
	BuildGitCmd(dir string, extraArgs ...string) *exec.Cmd
}

type ICmdBuilder interface {
	Runner
	BuildGitCmd(dir string, extraArgs ...string) *exec.Cmd
	BuildMakepkgCmd(dir string, extraArgs ...string) *exec.Cmd
	BuildPacmanCmd(args *parser.Arguments, mode parser.TargetMode, noConfirm bool) *exec.Cmd
	AddMakepkgFlag(string)
	SetPacmanDBPath(string)
	SudoLoop()
}

type CmdBuilder struct {
	GitBin           string
	GitFlags         []string
	MakepkgFlags     []string
	MakepkgConfPath  string
	MakepkgBin       string
	SudoBin          string
	SudoFlags        []string
	SudoLoopEnabled  bool
	PacmanBin        string
	PacmanConfigPath string
	PacmanDBPath     string
	Runner           Runner
}

func (c *CmdBuilder) BuildGitCmd(dir string, extraArgs ...string) *exec.Cmd {
	args := make([]string, len(c.GitFlags), len(c.GitFlags)+len(extraArgs))
	copy(args, c.GitFlags)

	if dir != "" {
		args = append(args, "-C", dir)
	}

	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}

	cmd := exec.Command(c.GitBin, args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd
}

func (c *CmdBuilder) AddMakepkgFlag(flag string) {
	c.MakepkgFlags = append(c.MakepkgFlags, flag)
}

func (c *CmdBuilder) BuildMakepkgCmd(dir string, extraArgs ...string) *exec.Cmd {
	args := make([]string, len(c.MakepkgFlags), len(c.MakepkgFlags)+len(extraArgs))
	copy(args, c.MakepkgFlags)

	if c.MakepkgConfPath != "" {
		args = append(args, "--config", c.MakepkgConfPath)
	}

	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}

	cmd := exec.Command(c.MakepkgBin, args...)
	cmd.Dir = dir
	return cmd
}

func (c *CmdBuilder) SetPacmanDBPath(dbPath string) {
	c.PacmanDBPath = dbPath
}

func (c *CmdBuilder) BuildPacmanCmd(args *parser.Arguments, mode parser.TargetMode, noConfirm bool) *exec.Cmd {
	argArr := make([]string, 0, 32)
	needsRoot := args.NeedRoot(mode)

	if needsRoot {
		argArr = append(argArr, c.SudoBin)
		argArr = append(argArr, c.SudoFlags...)
	}

	argArr = append(argArr, c.PacmanBin)
	argArr = append(argArr, args.FormatGlobals()...)
	argArr = append(argArr, args.FormatArgs()...)
	if noConfirm {
		argArr = append(argArr, "--noconfirm")
	}

	argArr = append(argArr, "--config", c.PacmanConfigPath, "--")
	argArr = append(argArr, args.Targets...)

	if needsRoot {
		waitLock(c.PacmanDBPath)
	}
	return exec.Command(argArr[0], argArr[1:]...)
}

// waitLock will lock yay checking the status of db.lck until it does not exist
func waitLock(dbPath string) {
	lockDBPath := filepath.Join(dbPath, "db.lck")
	if _, err := os.Stat(lockDBPath); err != nil {
		return
	}

	text.Warnln(gotext.Get("%s is present.", lockDBPath))
	text.Warn(gotext.Get("There may be another Pacman instance running. Waiting..."))

	for {
		time.Sleep(3 * time.Second)
		if _, err := os.Stat(lockDBPath); err != nil {
			fmt.Println()
			return
		}
	}
}

func (c *CmdBuilder) SudoLoop() {
	c.updateSudo()
	go c.sudoLoopBackground()
}

func (c *CmdBuilder) sudoLoopBackground() {
	for {
		c.updateSudo()
		time.Sleep(SudoLoopDuration * time.Second)
	}
}

func (c *CmdBuilder) updateSudo() {
	for {
		err := c.Show(exec.Command(c.SudoBin, "-v"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			break
		}
	}
}

func (c *CmdBuilder) Show(cmd *exec.Cmd) error {
	return c.Runner.Show(cmd)
}

func (c *CmdBuilder) Capture(cmd *exec.Cmd, timeout int64) (stdout, stderr string, err error) {
	return c.Runner.Capture(cmd, timeout)
}
