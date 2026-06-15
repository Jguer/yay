# yay

**Yet Another Yogurt** — an AUR helper for Arch Linux, written in Go.

yay wraps pacman for official-repository packages and adds full AUR support:
PKGBUILD downloading, cross-source dependency resolution, makepkg-based
building, devel/VCS package tracking, and AUR voting. A single
pacman-compatible CLI for both sources.

## Install

### AUR (recommended)

Use the AUR helper you already have, or install manually:

```bash
git clone https://aur.archlinux.org/yay.git
cd yay && makepkg -si
```

### yay-bin (no Go required)

Skip the build entirely if you do not want to install Go:

```bash
git clone https://aur.archlinux.org/yay-bin.git
cd yay-bin && makepkg -si
```

### yay-git (latest git)

Track the latest development branch from git:

```bash
git clone https://aur.archlinux.org/yay-git.git
cd yay-git && makepkg -si
```

## Documentation

- [Manual — yay(8)](man.html) — complete command and option reference
- [Lua API](lua.html) — init.lua hooks, autocmds, and configuration
- [init.lua template](init-lua.html) — ready-to-copy configuration template

## Quick reference

```bash
yay foo                     # search and install (yogurt mode)
yay -Syu                    # full upgrade: repo + AUR
yay -Syua                   # AUR-only upgrade
yay -G foo                  # download PKGBUILD
yay -Ps                     # system statistics and health check
```

## Source

[github.com/Jguer/yay](https://github.com/Jguer/yay)
