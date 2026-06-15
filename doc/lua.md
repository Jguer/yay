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

The entire search result list can be rendered by your own Lua function. Register
a callback with `yay.on(event, fn)`. `event` must be a supported event name;
passing any other string aborts startup (fail-fast, like an unknown `yay.opt` key).

```lua
yay.on("render_search", function(results)
  local out = {}
  for _, pkg in ipairs(results) do
    local prefix = pkg.index and (pkg.index .. " ") or ""
    out[#out + 1] = string.format("%s%s/%s %s", prefix, pkg.source, pkg.name, pkg.version)
  end
  return table.concat(out, "\n")
end)
```

### Events

- `"render_search"` — fired **once** with the whole result list; the callback
  returns the entire menu output as a single string.

Hooks fire only in the detailed search view (`-Ss`) and the interactive number
menu (`yay <term>`). Name-only output (`-Sq`) and package info (`-Si`) are not
hooked, so machine-readable output stays stable. When there are no results the
hook is not called.

There is a single slot per callback; calling `yay.on("render_search", …)` again
replaces the previous registration (last-wins).

### Return contract

- Return a **string**: it is printed verbatim as the entire menu — yay prepends
  nothing, including number-menu indices. Reproduce each index from `pkg.index`
  if you want them.
- Return `nil`, nothing, or a non-string value: yay falls back to its built-in
  per-line formatter for the whole menu (the "defer to default" path).
- Raise a Lua `error(...)`: the search aborts and yay exits non-zero with
  `init.lua render_search hook: ...`.

No color/format helpers are exposed. Returned text is printed as-is; embed your
own ANSI escapes if you want styling, and honor `--color=never`/`NO_COLOR`
yourself.

### Result entry fields

Every element of `results` uses the same schema regardless of source (AUR or repo).
AUR-only numeric fields are `-1` for repo packages.

| key | type | notes |
| --- | --- | --- |
| `source` | string | `"aur"` or repo/DB name (e.g. `"extra"`) |
| `name` | string | |
| `version` | string | |
| `description` | string | |
| `package_base` | string | |
| `votes` | number | AUR vote count; **`-1` for repo packages** |
| `popularity` | number | AUR popularity; **`-1` for repo packages** |
| `first_submitted` | number | unix timestamp; **`-1` for repo packages** |
| `last_modified` | number | unix timestamp; **`-1` for repo packages** |
| `provides` | array of strings | possibly empty |
| `installed` | boolean | |
| `installed_version` | string | **`nil` when not installed** |
| `index` | number | 1-based selection number; **`nil` outside the number menu** |