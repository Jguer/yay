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

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/Jguer/yay/v12/pkg/settings"
	"github.com/Jguer/yay/v12/pkg/settings/parser"
	"github.com/Jguer/yay/v12/pkg/text"
)

const SudoLoopDuration = 241

var gitDenyList = mapset.NewThreadUnsafeSet(
	"GIT_WORK_TREE",
	"GIT_DIR",
)

type GitCmdBuilder interface {
	Runner
	BuildGitCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd
}

type ICmdBuilder interface {
	Runner
	BuildGitCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd
	BuildGPGCmd(ctx context.Context, extraArgs ...string) *exec.Cmd
	BuildMakepkgCmd(ctx context.Context, dir string, extraArgs ...string) *exec.Cmd
	BuildPacmanCmd(ctx context.Context, args *parser.Arguments, mode parser.TargetMode, noConfirm bool) *exec.Cmd
	AddMakepkgFlag(string)
	GetCleanBuild() bool
	SudoLoop()
}

type CmdBuilder struct {
	CleanBuild       bool
	GitBin           string
	GitFlags         []string
	GPGBin           string
	GPGFlags         []string
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
	Log              *text.Logger
}

func NewCmdBuilder(cfg *settings.Configuration, runner Runner, logger *text.Logger, dbPath string) *CmdBuilder {
	return &CmdBuilder{
		CleanBuild:       cfg.CleanBuild,
		GitBin:           cfg.GitBin,
		GitFlags:         strings.Fields(cfg.GitFlags),
		GPGBin:           cfg.GpgBin,
		GPGFlags:         strings.Fields(cfg.GpgFlags),
		MakepkgFlags:     strings.Fields(cfg.MFlags),
		MakepkgConfPath:  cfg.MakepkgConf,
		MakepkgBin:       cfg.MakepkgBin,
		SudoBin:          cfg.SudoBin,
		SudoFlags:        strings.Fields(cfg.SudoFlags),
		SudoLoopEnabled:  cfg.SudoLoop,
		PacmanBin:        cfg.PacmanBin,
		PacmanConfigPath: cfg.PacmanConf,
		PacmanDBPath:     dbPath,
		Runner:           runner,
		Log:              logger,
	}
}

func (c *CmdBuilder) BuildGPGCmd(ctx context.Context, extraArgs ...string) *exec.Cmd {
	args := make([]string, len(c.GPGFlags), len(c.GPGFlags)+len(extraArgs))
	copy(args, c.GPGFlags)

	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}

	cmd := exec.CommandContext(ctx, c.GPGBin, args...)

	cmd = c.deElevateCommand(ctx, cmd)

	return cmd
}

func gitFilteredEnv() []string {
	var env []string

	for _, envVar := range os.Environ() {
		envKey := strings.SplitN(envVar, "=", 2)[0]
		if !gitDenyList.Contains(envKey) {
			env = append(env, envVar)
		}
	}

	env = append(env, "GIT_TERMINAL_PROMPT=0")

	return env
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

	cmd.Env = gitFilteredEnv()

	cmd = c.deElevateCommand(ctx, cmd)

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

	cmd = c.deElevateCommand(ctx, cmd)

	return cmd
}

// deElevateCommand, `systemd-run` code based on pikaur.
func (c *CmdBuilder) deElevateCommand(ctx context.Context, cmd *exec.Cmd) *exec.Cmd {
	if os.Geteuid() != 0 {
		return cmd
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

		return cmd
	}

	cmdArgs := []string{
		"--service-type=oneshot",
		"--pipe", "--wait", "--pty", "--quiet",
		"-p", "DynamicUser=yes",
		"-p", "CacheDirectory=yay",
		"-E", "HOME=/tmp",
	}

	if cmd.Dir != "" {
		cmdArgs = append(cmdArgs, "-p", fmt.Sprintf("WorkingDirectory=%s", cmd.Dir))
	}

	for _, envVarName := range [...]string{"http_proxy", "https_proxy", "ftp_proxy"} {
		if env := os.Getenv(envVarName); env != "" {
			cmdArgs = append(cmdArgs, "-E", fmt.Sprintf("%s=%s", envVarName, env))
		}
	}

	path, _ := exec.LookPath(cmd.Args[0])

	cmdArgs = append(cmdArgs, path)
	cmdArgs = append(cmdArgs, cmd.Args[1:]...)

	systemdCmd := exec.CommandContext(ctx, "systemd-run", cmdArgs...)
	systemdCmd.Dir = cmd.Dir

	return systemdCmd
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
		c.waitLock(c.PacmanDBPath)

		if os.Geteuid() != 0 {
			return c.buildPrivilegeElevatorCommand(ctx, argArr)
		}
	}

	return exec.CommandContext(ctx, argArr[0], argArr[1:]...)
}

// waitLock will lock yay checking the status of db.lck until it does not exist.
func (c *CmdBuilder) waitLock(dbPath string) {
	lockDBPath := filepath.Join(dbPath, "db.lck")
	if _, err := os.Stat(lockDBPath); err != nil {
		return
	}

	c.Log.Warnln(gotext.Get("%s is present.", lockDBPath))
	c.Log.Warn(gotext.Get("There may be another Pacman instance running. Waiting..."))

	for {
		time.Sleep(3 * time.Second)

		if _, err := os.Stat(lockDBPath); err != nil {
			c.Log.Println()

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
			c.Log.Errorln(err)
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

func (c *CmdBuilder) GetCleanBuild() bool {
	return c.CleanBuild
}
