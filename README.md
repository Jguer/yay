# Yay

Yet Another Yogurt - An AUR Helper Written in Go

#### Packages

[![yay](https://img.shields.io/aur/version/yay?color=1793d1&label=yay&logo=arch-linux&style=for-the-badge)](https://aur.archlinux.org/packages/yay/)
[![yay-bin](https://img.shields.io/aur/version/yay-bin?color=1793d1&label=yay-bin&logo=arch-linux&style=for-the-badge)](https://aur.archlinux.org/packages/yay-bin/)
[![yay-git](https://img.shields.io/aur/version/yay-git?color=1793d1&label=yay-git&logo=arch-linux&style=for-the-badge)](https://aur.archlinux.org/packages/yay-git/)
![AUR votes](https://img.shields.io/aur/votes/yay?color=333333&style=for-the-badge)
[![GitHub license](https://img.shields.io/github/license/jguer/yay?color=333333&style=for-the-badge)](https://github.com/Jguer/yay/blob/master/LICENSE)

## Objectives

There's a point in everyone's life when you feel the need to write an AUR helper because there are only about 20 of them.
So say hi to 20+1.

Yay is based on the design of [yaourt](https://github.com/archlinuxfr/yaourt), [apacman](https://github.com/oshazard/apacman) and [pacaur](https://github.com/rmarquis/pacaur). It is developed with these objectives in mind:

- Provide an interface for pacman
- Yaourt-style interactive search/install
- Minimal dependencies
- Minimize user input
- Know when git packages are due for upgrades

## Features

- Perform advanced dependency solving
- Download PKGBUILDs from ABS or AUR
- Tab-complete the AUR
- Query user up-front for all input (prior to starting builds)
- Narrow search terms (`yay linux header` will first search `linux` and then narrow on `header`)
- Find matching package providers during search and allow selection
- Remove make dependencies at the end of the build process
- Run without sourcing PKGBUILD

## Installation

If you are migrating from another AUR helper, you can simply install Yay with that helper.

Alternatively, the initial installation of Yay can be done by cloning the PKGBUILD and
building with makepkg:

Before you begin, make sure you have the `base-devel` package group installed.

```sh
git clone https://aur.archlinux.org/yay.git
cd yay
makepkg -si
```

## Support

All support related to Yay should be requested via GitHub issues. Since Yay is not
officially supported by Arch Linux, support should not be sought out on the
forums, AUR comments or other official channels.

A broken AUR package should be reported as a comment on the package's AUR page.
A package may only be considered broken if it fails to build with makepkg.
Reports should be made using makepkg and include the full output as well as any
other relevant information. Never make reports using Yay or any other external
tools.

## Frequently Asked Questions

#### Yay does not display colored output. How do I fix it?

Make sure you have the `Color` option in your `/etc/pacman.conf`
(see issue [#123](https://github.com/Jguer/yay/issues/123)).

#### Yay is not prompting to skip packages during system upgrade.

The default behavior was changed after
[v8.918](https://github.com/Jguer/yay/releases/tag/v8.918)
(see [3bdb534](https://github.com/Jguer/yay/commit/3bdb5343218d99d40f8a449b887348611f6bdbfc)
and issue [#554](https://github.com/Jguer/yay/issues/554)).
To restore the package-skip behavior use `--combinedupgrade` (make
it permanent by appending `--save`). Note: skipping packages will leave your
system in a
[partially-upgraded state](https://wiki.archlinux.org/index.php/System_maintenance#Partial_upgrades_are_unsupported).

#### Sometimes diffs are printed to the terminal, and other times they are paged via less. How do I fix this?

Yay uses `git diff` to display diffs, which by default tells less not to
page if the output can fit into one terminal length. This behavior can be
overridden by exporting your own flags (`export LESS=SRX`).

#### Yay is not asking me to edit PKGBUILDS, and I don't like the diff menu! What can I do?

`yay --editmenu --nodiffmenu --save`

#### How can I tell Yay to act only on AUR packages, or only on repo packages?

`yay -{OPERATION} --aur`
`yay -{OPERATION} --repo`

#### An `Out Of Date AUR Packages` message is displayed. Why doesn't Yay update them?

This message does not mean that updated AUR packages are available. It means
the packages have been flagged out of date on the AUR, but
their maintainers have not yet updated the `PKGBUILD`s
(see [outdated AUR packages](https://wiki.archlinux.org/index.php/Arch_User_Repository#Foo_in_the_AUR_is_outdated.3B_what_should_I_do.3F)).

#### Yay doesn't install dependencies added to a PKGBUILD during installation.

Yay resolves all dependencies ahead of time. You are free to edit the
PKGBUILD in any way, but any problems you cause are your own and should not be
reported unless they can be reproduced with the original PKGBUILD.

#### I know my `-git` package has updates but yay doesn't offer to update it

Yay uses an hash cache for development packages. Normally it is updated at the end of the package install with the message `Found git repo`.
If you transition between aur helpers and did not install the devel package using yay at some point, it is possible it never got added to the cache. `yay -Y --gendb` will fix the current version of every devel package and start checking from there.

#### I want to help out!

Check [CONTRIBUTING.md](./CONTRIBUTING.md) for more information.

## Examples of Custom Operations

| Command                         | Description                                                                                                                                         |
| ------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `yay <Search Term>`             | Present package-installation selection menu.                                                                                                        |
| `yay -Ps`                       | Print system statistics.                                                                                                                            |
| `yay -Yc`                       | Clean unneeded dependencies.                                                                                                                        |
| `yay -G <AUR Package>`          | Download PKGBUILD from ABS or AUR.                                                                                                                  |
| `yay -Y --gendb`                | Generate development package database used for devel update.                                                                                        |
| `yay -Syu --devel --timeupdate` | Perform system upgrade, but also check for development package updates and use PKGBUILD modification time (not version number) to determine update. |

## Images

<p float="left">
<img src="https://rawcdn.githack.com/Jguer/jguer.github.io/77647f396cb7156fd32e30970dbeaf6d6dc7f983/yay/yay.png" width="42%"/>
<img src="https://rawcdn.githack.com/Jguer/jguer.github.io/77647f396cb7156fd32e30970dbeaf6d6dc7f983/yay/yay-s.png" width="42%"/>
</p>

<p float="left">
<img src="https://rawcdn.githack.com/Jguer/jguer.github.io/77647f396cb7156fd32e30970dbeaf6d6dc7f983/yay/yay-y.png" width="42%"/>
<img src="https://rawcdn.githack.com/Jguer/jguer.github.io/77647f396cb7156fd32e30970dbeaf6d6dc7f983/yay/yay-ps.png" width="42%"/>
</p>
