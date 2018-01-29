# yay
Yet another Yogurt - An AUR Helper written in Go

#### Packages
[![yay](https://img.shields.io/aur/version/yay.svg?label=yay)](https://aur.archlinux.org/packages/yay/) [![yay-bin](https://img.shields.io/aur/version/yay-bin.svg?label=yay-bin)](https://aur.archlinux.org/packages/yay-bin/) [![yay-git](https://img.shields.io/aur/version/yaygit.svg?label=yay-git)](https://aur.archlinux.org/packages/yay-git/) [![GitHub license](https://img.shields.io/badge/license-AGPL-blue.svg)](https://raw.githubusercontent.com/Jguer/yay/master/LICENSE)

There's a point in everyone's life when you feel the need to write an AUR helper because there are only about 20 of them.
So say hi to 20+1.

Yay was created with a few objectives in mind and based on the design of [yaourt](https://github.com/archlinuxfr/yaourt) and [apacman](https://github.com/oshazard/apacman):

- Have almost no dependencies.
- Provide an interface for pacman.
- Have yaourt like search.
- Minimize user input
- Know when git packages are due for an upgrade.

## Features
- AUR Tab completion
- Search narrowing (`yay linux header` will first search linux and then narrow on header)
- No sourcing of PKGBUILD is done
- The binary has no dependencies that pacman doesn't already have.
- Sources build dependencies
- Removes make dependencies at the end of build process

#### Custom Operations
- `yay <Search Term>` presents package selection menu
- `yay -Qstats` delivers system statistics
- `yay -Cd` cleans unneeded dependencies
- `yay -G` downloads PKGBuild from ABS or AUR
- `yay --gendb` generates development package DB used for devel updates.

<img src="http://jguer.github.io/yay/yayupgrade.png" width="450">
<img src="http://jguer.github.io/yay/yay2.png" width="450">
<img src="http://jguer.github.io/yay/yay3.png" width="450">

### Changelog
#### 2.219
- Updated manpage
- Updated --help
- Fixed AUR update fails with large number of packages #59 
- Check if package is already in upgrade list and skip it. #60 
-  Add -V and -h for flag parsing @AnthonyLam 
-  Prevent file corruption by truncating the files @maximbaz 
-  Print VCS error details @maximbaz 
-  Using '-' doesn't raise an error @PietroCarrara 
-  use Command.Dir in aur.PkgInstall; Fixes #32 #47  @afg984 
-  Suffix YayConf.BuildDir with uid to avoid permission issues @afg984 (Not included in last changelog)

#### 2.200
- Development github package support readded

#### 2.196
- XDG_CONFIG_HOME support
- XDG_CACHE_HOME support

#### 2.165
- Upgrade list now allows skipping upgrade install

#### 2.159
- Qstats now warns about packages not available in AUR

#### 2.152
- Fetching backend changed to Mikkel Oscar's [Aur](https://github.com/mikkeloscar/aur)
- Added support for development packages from github.
- Pacman backend rewritten and simplified
- Added config framework.

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
