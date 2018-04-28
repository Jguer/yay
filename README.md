# yay

Yet another Yogurt - An AUR Helper written in Go

#### Packages

[![yay](https://img.shields.io/aur/version/yay.svg?label=yay)](https://aur.archlinux.org/packages/yay/) [![yay-bin](https://img.shields.io/aur/version/yay-bin.svg?label=yay-bin)](https://aur.archlinux.org/packages/yay-bin/) [![yay-git](https://img.shields.io/aur/version/yay-git.svg?label=yay-git)](https://aur.archlinux.org/packages/yay-git/) [![GitHub license](https://img.shields.io/github/license/jguer/yay.svg)](https://github.com/Jguer/yay/blob/master/LICENSE)  
There's a point in everyone's life when you feel the need to write an AUR helper because there are only about 20 of them.
So say hi to 20+1.

Yay was created with a few objectives in mind and based on the design of [yaourt](https://github.com/archlinuxfr/yaourt) and [apacman](https://github.com/oshazard/apacman):

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
* Sources build dependencies
* Removes make dependencies at the end of build process

#### Frequently Asked Questions

* Yay does not display colored output. How do I fix it?  
  Make sure you have the `Color` option in your `/etc/pacman.conf` [#123](https://github.com/Jguer/yay/issues/123)

#### Example of Custom Operations

* `yay <Search Term>` presents package selection menu
* `yay -Ps` prints system statistics
* `yay -Pu` prints update list
* `yay -Yc` cleans unneeded dependencies
* `yay -G` downloads PKGBUILD from ABS or AUR
* `yay -Y --gendb` generates development package DB used for devel updates.
* `yay -Syu --devel --timeupdate` Normal update but also check for development
  package updates and uses PKGBUILD modification time and not version to
  determine update

<img src="https://cdn.rawgit.com/Jguer/jguer.github.io/5412b8d6/yay/yay-ps.png" width="450">
<img src="https://cdn.rawgit.com/Jguer/jguer.github.io/5412b8d6/yay/yayupgrade.png" width="450">
<img src="https://cdn.rawgit.com/Jguer/jguer.github.io/5412b8d6/yay/yaysearch.png" width="450">
