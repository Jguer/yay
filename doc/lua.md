# Lua configuration (`init.lua`)

yay can optionally load a Lua configuration file, `init.lua`. `init.lua` overlays whatever is in `config.json`, and any command-line flag you pass still wins over `init.lua`.

## Location

`init.lua` is looked up, in order:

1. `$XDG_CONFIG_HOME/yay/init.lua`
2. `$HOME/.config/yay/init.lua`

## Setting options with `yay.opt`

Assign to keys on the `yay.opt` table using the exact option names shown
below.

```lua
yay.opt.bottom_up = false
yay.opt.clean_after = true
yay.opt.sort_by = "votes"
yay.opt.request_split_n = 150
yay.opt.editor = os.getenv("EDITOR") or "vi"
```

Unknown keys and values of the wrong type are treated as errors. yay aborts
startup and reports the offending keys/values so misconfigurations fail fast.

### Available options

**Strings**

`aururl`, `aurrpcurl`, `build_dir`, `editor`, `editor_flags`, `makepkg_bin`,
`makepkg_conf`, `pacman_bin`, `pacman_conf`, `redownload`, `rebuild`, `git_bin`,
`gpg_bin`, `gpg_flags`, `mflags`, `sort_by`, `search_by`, `git_flags`,
`remove_make`, `sudo_bin`, `sudo_flags`

**Integers**

`request_split_n`, `completion_refresh_time`, `max_concurrent_downloads`

**Booleans**

`bottom_up`, `sudo_loop`, `devel`, `clean_after`, `keep_src`, `provides`,
`pgp_fetch`, `clean_menu`, `diff_menu`, `edit_menu`, `combined_upgrade`,
`use_ask`, `batch_install`, `single_line_results`, `separate_sources`, `debug`,
`rpc`, `double_confirm`

A ready-to-copy example
lives at [`doc/init.lua`](init.lua).

## Logging with `yay.log`

Lua config and hooks can write through yay's normal logger:

```lua
yay.log.debug("build dir:", yay.opt.build_dir)
yay.log.info("loaded init.lua")
yay.log.warn("skipping", "pkgname")
yay.log.error("policy check failed")
```

`debug` only prints when debug logging is enabled. `error` logs an error-level
message and does not stop execution; use `yay.abort("message")` for controlled
hook stops.

## Upgrade selection hooks

`UpgradeSelect` runs during `yay -Syu` after yay has built and sorted the
upgrade graph, and before the native "Packages to exclude" menu is printed.
The hook can return package names to exclude. By default, yay still shows the
native menu after applying hook exclusions.

```lua
yay.create_autocmd("UpgradeSelect", {
  desc = "skip recently modified AUR upgrades",
  callback = function(event)
    local exclude = {}
    local recent_cutoff = os.time() - (3 * 24 * 60 * 60)
    for _, pkg in ipairs(event.data.upgrades) do
      if pkg.repository == "aur" and pkg.last_modified >= recent_cutoff then
        yay.log.debug("pre-excluding recently modified AUR package:", pkg.name)
        table.insert(exclude, pkg.name)
      end
    end

    return { exclude = exclude, skip_menu = false }
  end,
})
```

Multiple `UpgradeSelect` hooks run in registration order. Their `exclude`
lists are unioned. If any hook returns `skip_menu = true`, yay applies all hook
exclusions and skips the native menu. With `skip_menu = false` or no return
value, hook exclusions are applied first and then the native menu is shown.

Returned exclusions must name packages from `event.data.upgrades`. Unknown
names are treated as hook errors so typos do not silently upgrade the wrong
package. Pulled dependencies are visible in `event.data.pulled_dependencies`,
but they are removed only when pruning an excluded upgrade candidate requires
it.

### UpgradeSelect event

The callback receives this table:

```lua
{
  event = "UpgradeSelect",
  data = {
    upgrades = {
      {
        id = 3,
        name = "pkgname",
        base = "pkgbase",
        repository = "aur",
        local_version = "1.2.3-3",
        remote_version = "1.2.3-4",
        reason = "explicit",
        last_modified = 1700000000,
        maintainer = "username",
      },
    },
    pulled_dependencies = {
      {
        id = 0,
        name = "depname",
        base = "",
        repository = "core",
        local_version = "",
        remote_version = "1.0-1",
        reason = "dependency",
        last_modified = 0,
        maintainer = "",
      },
    },
  },
}
```

For selectable `data.upgrades` entries, `id` matches the number shown in the
native menu. `pulled_dependencies` entries are shown separately by yay and use
`id = 0` because they are not directly selectable.

## AUR pre-install hooks

`init.lua` can register hooks with a small autocmd API:

```lua
yay.create_autocmd("AURPreInstall", {
  desc = "inspect or modify AUR package files",
  callback = function(event)
    -- event.match is the package base.
    -- event.data has package metadata and local file paths.
  end,
})
```

`AURPreInstall` runs once per AUR package base, in sorted package-base order,
after the AUR PKGBUILD repositories are downloaded and merged. It runs before
the clean, diff, and edit menus, and before source downloads or builds.

Use `yay.abort("message")` for controlled policy stops without a Lua
traceback. If a callback raises a Lua error, yay aborts the install before
build work starts and includes the Lua traceback for debugging.

Changing fields in the Lua `event` table does not change yay's internal
package state. Hooks can still edit files through Lua's normal `io` and `os`
libraries; later menus and build steps will see those file changes.

### AURPreInstall event

The callback receives this table:

```lua
{
  event = "AURPreInstall",
  match = "pkgbase",
  data = {
    base = "pkgbase",
    dir = "/path/to/build/pkgbase",
    pkgbuild_path = "/path/to/build/pkgbase/PKGBUILD",
    srcinfo_path = "/path/to/build/pkgbase/.SRCINFO",
    pkgbuild = "...PKGBUILD contents...",
    version = "1:1.2.3-4",
    last_modified = 1700000000,
    installed = true,
    packages = {
      {
        name = "pkgname",
        version = "1:1.2.3-4",
        local_version = "1:1.2.3-3",
        reason = "explicit",
        upgrade = true,
        devel = false,
      },
    },
    srcinfo = {
      pkgbase = "pkgbase",
      pkgver = "1.2.3",
      pkgrel = "4",
      epoch = "1",
      version = "1:1.2.3-4",
      pkgdesc = "description",
      url = "https://example.invalid",
      arch = { "x86_64" },
      license = { "MIT" },
      depends = { "glibc" },
      makedepends = { "go" },
      checkdepends = { "bats" },
      optdepends = { "pkg: optional feature" },
      provides = { "virtual-pkg" },
      conflicts = { "old-pkg" },
      replaces = { "older-pkg" },
    },
  },
}
```

`data.packages` contains the target packages for that base. Split packages are
listed separately. `reason` is one of `explicit`, `dependency`,
`make_dependency`, `check_dependency`, or `unknown`.

### Example

```lua
yay.create_autocmd("AURPreInstall", {
  desc = "block forbidden sources and patch a PKGBUILD",
  callback = function(event)
    if event.data.pkgbuild:match("forbidden.example") then
      yay.log.warn(event.match .. ": forbidden source URL")
      yay.abort(event.match .. ": forbidden source URL")
    end

    if event.match == "demo-pkg" then
      local path = event.data.pkgbuild_path
      local f = assert(io.open(path, "r"))
      local body = f:read("*a")
      f:close()

      body = body:gsub("options=%('strip'%)", "options=('!strip')")

      f = assert(io.open(path, "w"))
      f:write(body)
      f:close()
    end
  end,
})
```

## AUR post-download hooks

`AURPostDownload` runs once per AUR package base, in sorted package-base order,
after yay runs `makepkg --verifysource` for package sources and before
compatibility checks, PGP key import prompts, builds, or package installs.

Use `yay.abort("message")` to stop the operation without a Lua traceback.
`AURPostDownload` receives the same payload shape as `AURPreInstall`; only the
`event` value differs.

### AURPostDownload event

The callback receives this table:

```lua
{
  event = "AURPostDownload",
  match = "pkgbase",
  data = {
    base = "pkgbase",
    dir = "/path/to/build/pkgbase",
    pkgbuild_path = "/path/to/build/pkgbase/PKGBUILD",
    srcinfo_path = "/path/to/build/pkgbase/.SRCINFO",
    pkgbuild = "...PKGBUILD contents...",
    version = "1:1.2.3-4",
    last_modified = 1700000000,
    installed = true,
    packages = { ... },
    srcinfo = { ... },
  },
}
```

### Example

```lua
yay.create_autocmd("AURPostDownload", {
  desc = "block forbidden source URLs after download",
  callback = function(event)
    if event.data.pkgbuild:match("forbidden.example") then
      yay.abort(event.match .. ": forbidden source URL")
    end
  end,
})
```
