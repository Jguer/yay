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

![Yay Qstats](http://jguer.github.io/yay/yay1.png "yay -Qstats")

### Changelog
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

