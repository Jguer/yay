# End-to-end tests

`run.sh` exercises PKGBUILD repository support (`yay.opt.pkgbuild_repos`) against
real `makepkg` and `pacman`, rather than mocks. It builds `yay` and then:

1. **`file://` repository** — installs a package from a local directory
   repository used in place.
2. **`-Sua` upgrade** — bumps that package and upgrades it from the repository.
3. **`git+file://` repository** — installs a package from a local git
   repository, which yay clones into its cache.

Each package is trivial (`arch=('any')`, no sources) and does not exist in the
AUR, so a successful install proves the package was resolved from the configured
repository. The test packages (`yay-e2e-file`, `yay-e2e-git`) are removed on
exit.

## Requirements

- An Arch Linux environment with `makepkg`, `pacman`, `fakeroot`, `git`.
- Passwordless `sudo` (yay runs `sudo pacman` to install built packages).
- A **non-root** user — `makepkg` refuses to run as root.

## Running

```sh
make test-e2e
# or, against an existing binary:
YAY_BIN=/path/to/yay ./e2e/run.sh
```

CI runs this via the `E2E PKGBUILD repositories` job in
`.github/workflows/testing.yml`, using the Arch-based `yay-builder` image and an
unprivileged user.
