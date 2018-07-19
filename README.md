# yay

Yet another Yogurt - An AUR Helper written in Go

#### Packages

[![yay](https://img.shields.io/aur/version/yay.svg?label=yay)](https://aur.archlinux.org/packages/yay/) [![yay-bin](https://img.shields.io/aur/version/yay-bin.svg?label=yay-bin)](https://aur.archlinux.org/packages/yay-bin/) [![yay-git](https://img.shields.io/aur/version/yay-git.svg?label=yay-git)](https://aur.archlinux.org/packages/yay-git/) [![GitHub license](https://img.shields.io/github/license/jguer/yay.svg)](https://github.com/Jguer/yay/blob/master/LICENSE)  
There's a point in everyone's life when you feel the need to write an AUR helper because there are only about 20 of them.
So say hi to 20+1.

Yay was created with a few objectives in mind and based on the design of [yaourt](https://github.com/archlinuxfr/yaourt), [apacman](https://github.com/oshazard/apacman) and [pacaur](https://github.com/rmarquis/pacaur):

* Have almost no dependencies.
* Provide an interface for pacman.
* Have yaourt like search.
* Minimize user input
* Know when git packages are due for an upgrade.

## Features

* AUR Tab completion
* Download PKGBUILD from ABS or AUR
* Ask all questions first and then start building
* Search narrowing (`yay linux header` will first search linux and then narrow on header)
* No sourcing of PKGBUILD is done
* The binary has no dependencies that pacman doesn't already have.
* Advanced dependency solving
* Remove make dependencies at the end of the build process

## Installation

If you are migrating from another AUR helper you can simply install Yay from
the AUR like any other package.

The initial installation of Yay can be done by cloning the PKGBUILD and
building with makepkg.
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

### Code Style

All code should be formatted through `go fmt`. This tool will automatically
format code for you. Although it is recommended you write code in this style
and just use this tool to catch mistakes.

### Building

Yay is easy to build with its only build dependency being `go` and the
assumption of `base-devel` being installed.

Run `make` to build Yay. This will generate a binary called `yay` in the same
directory as the Makefile.

Run `make test` to test Yay. This will check the code is formatted correctly,
run the code through `go vet` and run unit tests.

Yay's Makefile automatically sets the `GOPATH` to `$PWD/.go`. This makes it easy to
build using the dependencies in `vendor/`. Running manual go commands such as
`go build` will require that you to either set the `GOPATH` manually or `go get`
The dependencies into your own `GOPATH`.

### Vendored Dependencies

Yay depends on a couple of other projects. These are stored in `vendor/` and
are built into Yay at build time. They do not need to be installed separately.

Currently yay Depends on:

* https://github.com/Jguer/go-alpm
* https://github.com/Morganamilo/go-srcinfo
* https://github.com/mikkeloscar/aur

## Frequently Asked Questions

### Yay does not display colored output. How do I fix it?
  Make sure you have the `Color` option in your `/etc/pacman.conf` [#123](https://github.com/Jguer/yay/issues/123)

### Yay is not prompting to skip packages during sysupgrade (issue [#554](https://github.com/Jguer/yay/issues/554))
  The default behavior was changed after [v8.918](https://github.com/Jguer/yay/releases/tag/v8.918)
  (see: [3bdb534](https://github.com/Jguer/yay/commit/3bdb5343218d99d40f8a449b887348611f6bdbfc)).
  To restore such behavior use `--combinedupgrade`. This can also be
  permanently enabled by appending `--save`.
  Note: this causes [native pacman](https://wiki.archlinux.org/index.php/AUR_helpers) to become partial.

### Sometimes diffs are printed to the terminal and other times they are paged via less. How do I fix this?
  Yay uses `git diff` to display diffs, by default git tells less to not page
  if the output can fit one terminal length. This can be overridden by
  exporting your own flags `export LESS=SRX`.

### Yay is not asking me to edit PKGBUILDS and I don't like diff menu! What do?
  `yay --editmenu --nodiffmenu --save`

### Only act on AUR packages or only on repo packages?
  `yay -{OPERATION} --aur`
  `yay -{OPERATION} --repo`

### `Out Of Date AUR Packages` message is displayed, why doesn't `yay` update them?
  This means the package has been flagged out of date on the AUR but maintainer has not updated the `PKGBUILD` yet.

## Examples of Custom Operations

* `yay <Search Term>` presents package selection menu
* `yay -Ps` prints system statistics
* `yay -Pu` prints update list
* `yay -Yc` cleans unneeded dependencies
* `yay -G` downloads PKGBUILD from ABS or AUR
* `yay -Y --gendb` generates development package DB used for devel updates.
* `yay -Syu --devel --timeupdate` Normal update but also check for development
  package updates and uses PKGBUILD modification time and not version to
  determine update

## Images

<img src="https://cdn.rawgit.com/Jguer/jguer.github.io/5412b8d6/yay/yay-ps.png" width="450">
<img src="https://cdn.rawgit.com/Jguer/jguer.github.io/5412b8d6/yay/yayupgrade.png" width="450">
<img src="https://cdn.rawgit.com/Jguer/jguer.github.io/5412b8d6/yay/yaysearch.png" width="450">
