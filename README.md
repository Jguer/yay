# yay

Yet Another Yogurt - An AUR Helper Written in Go

#### Packages

[![yay](https://img.shields.io/aur/version/yay.svg?label=yay)](https://aur.archlinux.org/packages/yay/) [![yay-bin](https://img.shields.io/aur/version/yay-bin.svg?label=yay-bin)](https://aur.archlinux.org/packages/yay-bin/) [![yay-git](https://img.shields.io/aur/version/yay-git.svg?label=yay-git)](https://aur.archlinux.org/packages/yay-git/) [![GitHub license](https://img.shields.io/github/license/jguer/yay.svg)](https://github.com/Jguer/yay/blob/master/LICENSE)  

## Objectives

There's a point in everyone's life when you feel the need to write an AUR helper because there are only about 20 of them.
So say hi to 20+1.

_Yay_ is based on the design of [yaourt](https://github.com/archlinuxfr/yaourt), [apacman](https://github.com/oshazard/apacman) and [pacaur](https://github.com/rmarquis/pacaur). It is developed with these objectives in mind:

* Wrap the _pacman_ interface
* Search for packages like _yaourt_ does
* Minimize dependencies
* Minimize user input
* Know when git packages are due for upgrades

## Features

* Perform advanced dependency solving
* Download PKGBUILDs from ABS or AUR
* Tab-complete the AUR
* Query user up-front for all input (prior to starting builds)
* Narrow search terms (`yay linux header` will first search `linux` and then narrow on `header`)
* Remove make dependencies at the end of the build process
* Run without sourcing PKGBUILD
* Limit runtime dependencies to subset of _pacman_ dependencies

## Installation

If you are migrating from another AUR helper, you can simply install _yay_ with that helper.

If you are starting from scratch, you can install _yay_ by cloning the PKGBUILD and
then building with _makepkg_:
```sh
git clone https://aur.archlinux.org/yay.git
cd yay
makepkg -si
```

## Contributing

Contributors are always welcome!

If you plan to make any large changes or changes that may not be 100% agreed
on, we suggest opening an issue detailing your ideas first.

Otherwise send us a pull request and we will be happy to review it.

#### 1. Install Dependencies

Install `go` and `base-devel` with _pacman_.

Note that _yay_ requires a few other projects (as vendored dependencies). These
projects are stored in `vendor/`, are built into _yay_ at build time, and do not
need to be installed separately. Do not edit files in `vendor/`. Here are yay's
current vendored dependencies:

* https://github.com/Jguer/go-alpm
* https://github.com/Morganamilo/go-srcinfo
* https://github.com/mikkeloscar/aur

#### 2. Set GOPATH (As Needed)

_Yay_'s Makefile automatically sets the `GOPATH` to `$PWD/.go`. This path will
ensure dependencies in `vendor/` are built. Running manual _go_ commands such as
`go build` will require that you either set the `GOPATH` manually or `go get`
the vendored dependencies into your own `GOPATH`.

#### 3. Build

Run `make` to build _yay_. This command will generate a binary called `yay` in
the same directory as the Makefile.

#### 4. Check Style

All code should be formatted through `go fmt`. This tool will automatically
format code for you. We recommend, however, that you write code in the proper
style and use `go fmt` only to catch mistakes.

#### 5. Test

Run `make test` to test _yay_. This command will verify that the code is
formatted correctly, run the code through `go vet`, and run unit tests.

## Frequently Asked Questions

#### _Yay_ does not display colored output. How do I fix it?
  Make sure you have the `Color` option in your `/etc/pacman.conf`
  (see issue [#123](https://github.com/Jguer/yay/issues/123)).

#### _Yay_ is not prompting to skip packages during system upgrade.
  The default behavior was changed after
  [v8.918](https://github.com/Jguer/yay/releases/tag/v8.918)
  (see [3bdb534](https://github.com/Jguer/yay/commit/3bdb5343218d99d40f8a449b887348611f6bdbfc)
  and issue [#554](https://github.com/Jguer/yay/issues/554)).
  To restore the package-skip behavior use `--combinedupgrade` (make
  it permanent by appending `--save`). Note: skipping packages will leave your
  system in a
  [partially-upgraded state](https://wiki.archlinux.org/index.php/System_maintenance#Partial_upgrades_are_unsupported).

#### Sometimes diffs are printed to the terminal, and other times they are paged via _less_. How do I fix this?
  _Yay_ uses `git diff` to display diffs, which by default tells _less_ not to
  page if the output can fit into one terminal length. This behavior can be
  overridden by exporting your own flags (`export LESS=SRX`).

#### _Yay_ is not asking me to edit PKGBUILDS, and I don't like the diff menu! What can I do?
  `yay --editmenu --nodiffmenu --save`

#### How can I tell _yay_ to act only on AUR packages, or only on repo packages?
  `yay -{OPERATION} --aur`  
  `yay -{OPERATION} --repo`

#### An `Out Of Date AUR Packages` message is displayed. Why doesn't _yay_ update them?
  This message does not mean that updated AUR packages are available. It means
  means the packages have been flagged out of date on the AUR, but
  their maintainers have not yet updated the `PKGBUILD`s
  (see [outdated AUR packages](https://wiki.archlinux.org/index.php/Arch_User_Repository#Foo_in_the_AUR_is_outdated.3B_what_should_I_do.3F)).

#### _Yay_ doesn't install dependencies added to a PKGBUILD during installation.
  _Yay_ resolves all dependencies ahead of time. You are free to edit the
  PKGBUILD in any way, but any problems you cause are your own and should not be
  reported unless they can be reproduced with the original PKGBUILD.

## Examples of Custom Operations

* present package-installation selection menu  
  `yay <Search Term>`
* print system statistics  
  `yay -Ps`
* clean unneeded dependencies  
  `yay -Yc`
* download PKGBUILD from ABS or AUR  
  `yay -G <AUR Package>`
* generate development package database used for devel updates  
  `yay -Y --gendb`
* perform system upgrade, but also check for development package updates and use
  PKGBUILD modification time (not version number) to determine updates  
  `yay -Syu --devel --timeupdate`

## Images

<img src="https://cdn.rawgit.com/Jguer/jguer.github.io/5412b8d6/yay/yay-ps.png" width="450">
<img src="https://cdn.rawgit.com/Jguer/jguer.github.io/5412b8d6/yay/yayupgrade.png" width="450">
<img src="https://cdn.rawgit.com/Jguer/jguer.github.io/5412b8d6/yay/yaysearch.png" width="450">
