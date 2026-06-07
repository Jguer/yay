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
