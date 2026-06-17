package chroot

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Chroot struct {
	Sudo        string
	Path        string
	PacmanConf  string
	MakepkgConf string
	MFlags      []string
	Ro          []string
	Rw          []string
	RootPkgs    []string
}

func (c *Chroot) Exists() bool {
	p := filepath.Join(c.Path, "root")
	return p != "" && fileExists(p)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (c *Chroot) writePacmanConfTmp() (string, error) {
	f, err := os.CreateTemp("/tmp", "pacman.conf.*")
	if err != nil {
		return "", err
	}
	defer f.Close()

	in, err := os.Open(c.PacmanConf)
	if err != nil {
		return "", err
	}
	defer in.Close()

	// copy, but filter DBPath lines which may break pacstrap/mkarchroot
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "DBPath") {
			continue
		}
		if _, err := f.WriteString(line + "\n"); err != nil {
			return "", err
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	return f.Name(), nil
}

func (c *Chroot) Create() error {
	// create base dir
	args := []string{"install", "-dm755", c.Path}
	if c.Sudo != "" {
		cmd := exec.Command(c.Sudo, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	} else {
		if err := os.MkdirAll(c.Path, 0o755); err != nil {
			return err
		}
	}

	tmp, err := c.writePacmanConfTmp()
	if err != nil {
		return err
	}

	dir := filepath.Join(c.Path, "root")

	// mkarchroot -C tmp [-M makepkg_conf] dir [pkgs...]
	mkargs := []string{"-C", tmp}
	if c.MakepkgConf != "" {
		mkargs = append(mkargs, "-M", c.MakepkgConf)
	}
	mkargs = append(mkargs, dir)
	// ensure at least one package is provided (mkarchroot requires it)
	if len(c.RootPkgs) == 0 {
		c.RootPkgs = []string{"base-devel"}
	}
	mkargs = append(mkargs, c.RootPkgs...)

	var cmd *exec.Cmd
	if c.Sudo != "" {
		cmd = exec.Command(c.Sudo, append([]string{"mkarchroot"}, mkargs...)...)
	} else {
		cmd = exec.Command("mkarchroot", mkargs...)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkarchroot failed: %w", err)
	}

	return nil
}

func (c *Chroot) Update() error {
	// Run pacman -Syu inside the chroot using the same Run helper so
	// the chroot path and pacman.conf handling are applied consistently.
	return c.Run([]string{"pacman", "-Syu", "--noconfirm"})
}

func (c *Chroot) Run(args []string) error {
	tmp, err := c.writePacmanConfTmp()
	if err != nil {
		return err
	}

	cmdArgs := []string{"arch-nspawn", "-C", tmp, "-M", c.MakepkgConf, filepath.Join(c.Path, "root")}

	for _, f := range c.Ro {
		cmdArgs = append(cmdArgs, "--bind-ro", f)
	}
	for _, f := range c.Rw {
		cmdArgs = append(cmdArgs, "--bind", f)
	}

	cmdArgs = append(cmdArgs, args...)

	var cmd *exec.Cmd
	// If a privilege elevator is configured, run via it, otherwise run arch-nspawn directly
	if c.Sudo != "" {
		cmd = exec.Command(c.Sudo, cmdArgs...)
	} else {
		cmd = exec.Command(cmdArgs[0], cmdArgs[1:]...)
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
