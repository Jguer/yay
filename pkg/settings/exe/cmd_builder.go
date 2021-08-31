package exe

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v10/pkg/settings/parser"
	"github.com/Jguer/yay/v10/pkg/text"
)

const SudoLoopDuration = 241

type GitCmdBuilder interface {
	Runner
	BuildGitCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd
}

type ICmdBuilder interface {
	Runner
	BuildGitCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd
	BuildMakepkgCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd
	BuildPacmanCmd(ctx context.Context, args *parser.Arguments, mode parser.TargetMode, noConfirm bool) *exec.Cmd
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

func (c *CmdBuilder) BuildGitCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd {
	args := make([]string, len(c.GitFlags), len(c.GitFlags)+len(extraArgs))
	copy(args, c.GitFlags)

	if dir != "" {
		args = append(args, "-C", dir)
	}

	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}

	cmd := exec.CommandContext(ctx, c.GitBin, args...)

	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	c.deElevateCommand(cmd)

	return cmd
}

func (c *CmdBuilder) AddMakepkgFlag(flag string) {
	c.MakepkgFlags = append(c.MakepkgFlags, flag)
}

func (c *CmdBuilder) BuildMakepkgCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd {
	args := make([]string, len(c.MakepkgFlags), len(c.MakepkgFlags)+len(extraArgs))
	copy(args, c.MakepkgFlags)

	if c.MakepkgConfPath != "" {
		args = append(args, "--config", c.MakepkgConfPath)
	}

	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}

	cmd := exec.CommandContext(ctx, c.MakepkgBin, args...)
	cmd.Dir = dir

	c.deElevateCommand(cmd)

	return cmd
}

func (c *CmdBuilder) SetPacmanDBPath(dbPath string) {
	c.PacmanDBPath = dbPath
}

func (c *CmdBuilder) deElevateCommand(cmd *exec.Cmd) {
	if os.Geteuid() != 0 {
		return
	}

	ogCaller := ""
	if caller := os.Getenv("SUDO_USER"); caller != "" {
		ogCaller = caller
	} else if caller := os.Getenv("DOAS_USER"); caller != "" {
		ogCaller = caller
	}

	if userFound, err := user.Lookup(ogCaller); err == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		uid, _ := strconv.Atoi(userFound.Uid)
		gid, _ := strconv.Atoi(userFound.Gid)
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	}
}

func (c *CmdBuilder) buildPrivilegeElevatorCommand(ctx context.Context, ogArgs []string) *exec.Cmd {
	if c.SudoBin == "su" {
		return exec.CommandContext(ctx, c.SudoBin, "-c", strings.Join(ogArgs, " "))
	}

	argArr := make([]string, 0, len(c.SudoFlags)+len(ogArgs))
	argArr = append(argArr, c.SudoFlags...)
	argArr = append(argArr, ogArgs...)

	return exec.CommandContext(ctx, c.SudoBin, argArr...)
}

func (c *CmdBuilder) BuildPacmanCmd(ctx context.Context, args *parser.Arguments, mode parser.TargetMode, noConfirm bool) *exec.Cmd {
	argArr := make([]string, 0, 32)
	needsRoot := args.NeedRoot(mode)

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

		if os.Geteuid() != 0 {
			return c.buildPrivilegeElevatorCommand(ctx, argArr)
		}
	}

	return exec.CommandContext(ctx, argArr[0], argArr[1:]...)
}

// waitLock will lock yay checking the status of db.lck until it does not exist.
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

func (c *CmdBuilder) Capture(cmd *exec.Cmd) (stdout, stderr string, err error) {
	return c.Runner.Capture(cmd)
}
