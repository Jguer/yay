# yay
Yet another Yogurt - An AUR Helper written in Go

**Packages**
[![yay](https://img.shields.io/aur/version/yay.svg?label=yay)](https://aur.archlinux.org/packages/yay/) - Compile from source
[![yay-bin](https://img.shields.io/aur/version/yay-bin.svg?label=yay-bin)](https://aur.archlinux.org/packages/yay-bin/) - Binary version

There's a point in everyone's life when you feel the need to write an AUR helper because there are only about 20 of them.
So say hi to 20+1.

Yay was created with a few objectives in mind and based on the design of yaourt and apacman:

- Have almost no dependencies.
- Provide an interface for pacman.
- Have yaourt like search.
- Know when git packages are due for an upgrade (missing this one for now).

![Yay Qstats](http://jguer.github.io/yay/yay2.png "yay -Qstats")
![Yay NumberMenu](http://jguer.github.io/yay/yay3.png "yay gtk-theme")

### Custom Operations

- `yay <Search Term>` presents package selection menu
- `yay -Qstats` delivers system statistics
- `yay -Cd` cleans unneeded dependencies

### Changelog

#### 1.115
- Added AUR completions (updates on first completion every 48h)

#### 1.101
- Search speed and quality improved [#3](https://github.com/Jguer/yay/issues/3)

#### 1.100
- Added manpage
- Improved search [#3](https://github.com/Jguer/yay/issues/3)
- Added -G to get pkgbuild from the AUR or ABS. [#6](https://github.com/Jguer/yay/issues/6)
- Fixed [#8](https://github.com/Jguer/yay/issues/8)
- Completed and decluttered zsh completions
- If `$EDITOR` or `$VISUAL` is not set yay will prompt you for an editor [#7](https://github.com/Jguer/yay/issues/7)

#### 1.91
- `--downtop` has been replaced with `--bottomup` (as is logical)
- `yay -Ssq` and `yay -Sqs` now displays AUR packages with less information
- Repository search now uses the same criteria as pacman

#### 1.85
- yay now does -Si for AUR packages
- Fixed package install bugs

#### 1.83
- Added new dependency resolver for future features
- Sort package statistics

#### 1.80
- yay now warns when installing orphan packages
- Added orphan status to number menu
- Qstats now checks if system has orphan packages installed

#### 1.78
- Added foreign package statistics to Qstats
- Group installing is now possible
- Better handling of package dependency installing

#### 1.76
- Fixed critical bug that prevented AUR dependencies from being installed.

#### 1.70
- Stable for everyday use
- Bottom up package display
- Number menu like yaourt/apacman
- System package statistics

