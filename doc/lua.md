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

## Hooks (`yay.on`)

Search-result lines can be rendered by your own Lua function. Register a
callback with `yay.on(event, fn)`. `event` is a string naming one of the
supported events; passing any other event name aborts startup (fail-fast,
like an unknown `yay.opt` key).

```lua
yay.on("search_aur", function(pkg)
  local prefix = pkg.index and (pkg.index .. " ") or ""
  return string.format("%saur/%s %s (+%d %.2f)",
    prefix, pkg.name, pkg.version, pkg.votes, pkg.popularity)
end)
```

### Events

- `"search_aur"` — fired once per AUR result.
- `"search_repo"` — fired once per repo (sync) result.

Hooks fire only in the detailed search view (`-Ss`) and the interactive number
menu (`yay <term>`). Name-only output (`-Sq`) and package info (`-Si`) are not
hooked, so machine-readable output stays stable.

Each event holds a single callback; calling `yay.on` again for the same event
replaces the previous one (last-wins).

### Return contract

- Return a **string**: it is printed verbatim as the entire line — yay prepends
  nothing, including the number-menu index. Reproduce the index yourself from
  `pkg.index` if you want it.
- Return `nil`, nothing, or a non-string value: yay falls back to its built-in
  formatter for that row (the documented "defer to default" path).
- Raise a Lua `error(...)`: the search aborts and yay exits non-zero with
  `init.lua <event> hook: ...`.

No color/format helpers are exposed. Returned text is printed as-is; embed your
own ANSI escapes if you want styling, and honor `--color=never`/`NO_COLOR`
yourself.

### `search_aur` fields

| key | type | notes |
| --- | --- | --- |
| `source` | string | always `"aur"` |
| `name` | string | |
| `version` | string | |
| `description` | string | |
| `votes` | number | |
| `popularity` | number | |
| `out_of_date` | number | unix timestamp; `0` when not flagged |
| `last_modified` | number | unix timestamp of last AUR package modification |
| `package_base` | string | |
| `provides` | array of strings | possibly empty |
| `maintainer` | string | **`nil` when orphaned** |
| `installed` | boolean | |
| `installed_version` | string | **`nil` when not installed** |
| `count` | number | total number of result rows |
| `index` | number | 1-based selection number; **`nil` outside the number menu** |

### `search_repo` fields

| key | type | notes |
| --- | --- | --- |
| `source` | string | repo/DB name, e.g. `"extra"` |
| `name` | string | |
| `version` | string | |
| `description` | string | |
| `size` | number | download size in bytes |
| `installed_size` | number | installed size in bytes |
| `groups` | array of strings | possibly empty |
| `installed` | boolean | |
| `installed_version` | string | **`nil` when not installed** |
| `count` | number | total number of result rows |
| `index` | number | 1-based selection number; **`nil` outside the number menu** |
