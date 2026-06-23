-- yay Lua API type definitions for lua-language-server.
--
-- This is a meta file: lua-language-server loads it for type information only
-- and never executes it. Add this directory to your `workspace.library` so the
-- `yay` global, its options, and autocmd event payloads are recognised and
-- type-checked in your init.lua. See doc/lua.md "Editor support" for setup.
--
-- API reference: https://github.com/Jguer/yay/blob/next/doc/lua.md

---@meta

-- Aliases

---@alias yay.menuAnswer "" | "All" | "None" | "Installed" | "NotInstalled" | "abort"

---@alias yay.Event "AURPreInstall" | "AURPostDownload" | "UpgradeSelect" | "PostInstall" | "SearchFilter"

-- Options: yay.opt

---@class yay.opt
---@field aururl string Base AUR URL
---@field aurrpcurl string AUR RPC endpoint URL; empty uses default endpoint.
---@field build_dir string Build/cache directory for AUR packages.
---@field editor string Editor command used for PKGBUILD edits; empty uses VISUAL/EDITOR.
---@field editor_flags string Extra flags passed to the editor command.
---@field makepkg_bin string makepkg executable (name in PATH or absolute path).
---@field makepkg_conf string makepkg.conf path; empty uses default makepkg config.
---@field pacman_bin string pacman executable.
---@field pacman_conf string pacman.conf file path.
---@field redownload "no" | "yes" | "all" PKGBUILD download mode.
---@field git_bin string git executable.
---@field gpg_bin string gpg executable.
---@field gpg_flags string Extra flags passed to gpg.
---@field mflags string Extra flags passed to makepkg.
---@field sort_by "votes" | "popularity" | "name" | "base" | "submitted" | "modified" | "" AUR search sort field.
---@field search_by "name" | "name-desc" | "maintainer" | "submitter" | "depends" | "makedepends" | "optdepends" | "checkdepends" | "provides" | "conflicts" | "replaces" | "groups" | "keywords" | "comaintainers" AUR search field.
---@field git_flags string Extra flags passed to git.
---@field remove_make "no" | "yes" | "ask" | "askyes" Remove makedepends mode.
---@field sudo_bin string Privilege elevation command.
---@field sudo_flags string Extra flags passed to the sudo command.
---@field rebuild "no" | "yes" | "tree" | "all" Build mode.
---@field answer_clean yay.menuAnswer yay v13.0.1+ Pre-select clean menu answer (also accepts menu syntax: ranges, ^n).
---@field answer_diff yay.menuAnswer yay v13.0.1+ Pre-select diff menu answer (also accepts menu syntax: ranges, ^n).
---@field answer_edit yay.menuAnswer yay v13.0.1+ Pre-select edit menu answer (also accepts menu syntax: ranges, ^n).
---@field request_split_n integer Max packages per AUR RPC request (use values > 0).
---@field completion_refresh_time integer Completion cache refresh days: -1 (never), 0 (always), >0 (every N days).
---@field max_concurrent_downloads integer Parallel PKGBUILD source downloads; 0 uses CPU count.
---@field bottom_up boolean Show AUR packages before repo packages in mixed results.
---@field sudo_loop boolean Keep sudo session alive in the background during long builds.
---@field devel boolean Check development/VCS packages on sysupgrade.
---@field clean_after boolean Remove untracked files after install.
---@field keep_src boolean Keep pkg/ and src/ after successful builds.
---@field provides boolean Resolve matching providers when dependencies are ambiguous.
---@field pgp_fetch boolean Prompt to import unknown PGP keys from validpgpkeys.
---@field clean_menu boolean Show pre-build clean menu.
---@field diff_menu boolean Show diff menu before building.
---@field edit_menu boolean Show PKGBUILD edit menu before building.
---@field combined_upgrade boolean Use combined repo+AUR upgrade flow on sysupgrade.
---@field use_ask boolean Use pacman's --ask to auto-confirm known conflicts.
---@field batch_install boolean Queue AUR package installs instead of installing each package immediately.
---@field single_line_results boolean Use single-line search result format.
---@field separate_sources boolean Separate query results by source (repo vs AUR).
---@field debug boolean Enable debug logging and local init.lua lookup convenience.
---@field rpc boolean Use AUR RPC for dependency/query operations.
---@field double_confirm boolean Ask for confirmation before and after builds during upgrades.

-- Logging: yay.log

---@class yay.log
---@field debug fun(...: any)
---@field info fun(...: any)
---@field warn fun(...: any)
---@field error fun(...: any)

-- Event payloads: AURPreInstall / AURPostDownload
-- Both events share the same data shape; only the `event` string differs.

---@class yay.AURPreInstallPackage
---@field name string
---@field version string
---@field local_version string
---@field reason string
---@field upgrade boolean
---@field devel boolean

---@class yay.AURPreInstallSRCINFO
---@field pkgbase string
---@field pkgver string
---@field pkgrel string
---@field epoch string
---@field version string
---@field pkgdesc string
---@field url string
---@field arch string[]
---@field license string[]
---@field depends string[]
---@field makedepends string[]
---@field checkdepends string[]
---@field optdepends string[]
---@field provides string[]
---@field conflicts string[]
---@field replaces string[]

---@class yay.AURInstallData
---@field base string
---@field dir string
---@field pkgbuild_path string
---@field srcinfo_path string
---@field pkgbuild string
---@field version string
---@field last_modified integer
---@field installed boolean
---@field packages yay.AURPreInstallPackage[]
---@field srcinfo yay.AURPreInstallSRCINFO

---@class yay.AURPreInstallEvent
---@field event "AURPreInstall"
---@field match string
---@field data yay.AURInstallData

---@class yay.AURPostDownloadEvent
---@field event "AURPostDownload"
---@field match string
---@field data yay.AURInstallData

-- Event payloads: UpgradeSelect

---@class yay.UpgradeSelectPackage
---@field id integer
---@field name string
---@field base string
---@field repository string
---@field local_version string
---@field remote_version string
---@field reason string
---@field last_modified integer
---@field maintainer string

---@class yay.UpgradeSelectData
---@field upgrades yay.UpgradeSelectPackage[]
---@field pulled_dependencies yay.UpgradeSelectPackage[]

---@class yay.UpgradeSelectEvent
---@field event "UpgradeSelect"
---@field data yay.UpgradeSelectData

---@class yay.UpgradeSelectResult
---@field exclude string[]
---@field skip_menu boolean

-- Event payloads: PostInstall

---@class yay.PostInstallPackage
---@field name string
---@field version string
---@field local_version string
---@field source string
---@field reason string

---@class yay.PostInstallData
---@field packages yay.PostInstallPackage[]

---@class yay.PostInstallEvent
---@field event "PostInstall"
---@field data yay.PostInstallData

-- Event payloads: SearchFilter

---@class yay.SearchResultPackage
---@field source string
---@field name string
---@field description string
---@field base string
---@field votes integer
---@field popularity number
---@field first_submitted integer
---@field last_modified integer

---@class yay.SearchFilterData
---@field results yay.SearchResultPackage[]

---@class yay.SearchFilterEvent
---@field event "SearchFilter"
---@field data yay.SearchFilterData

---@class yay.SearchResultRef
---@field source string
---@field name string

-- create_autocmd opts: one per event so the callback payload is typed.

---@class yay.AURPreInstallOpts
---@field desc? string
---@field callback fun(event: yay.AURPreInstallEvent)

---@class yay.AURPostDownloadOpts
---@field desc? string
---@field callback fun(event: yay.AURPostDownloadEvent)

---@class yay.UpgradeSelectOpts
---@field desc? string
---@field callback fun(event: yay.UpgradeSelectEvent): yay.UpgradeSelectResult?

---@class yay.PostInstallOpts
---@field desc? string
---@field callback fun(event: yay.PostInstallEvent)

---@class yay.SearchFilterOpts
---@field desc? string
---@field callback fun(event: yay.SearchFilterEvent): yay.SearchResultRef[]?

---@overload fun(event: "AURPreInstall", opts: yay.AURPreInstallOpts)
---@overload fun(event: "AURPostDownload", opts: yay.AURPostDownloadOpts)
---@overload fun(event: "UpgradeSelect", opts: yay.UpgradeSelectOpts)
---@overload fun(event: "PostInstall", opts: yay.PostInstallOpts)
---@overload fun(event: "SearchFilter", opts: yay.SearchFilterOpts)
---@class yay.create_autocmd

-- The yay global

---@class yay
---@field opt? yay.opt
---@field log? yay.log
---@field abort? fun(reason: string)
---@field create_autocmd? yay.create_autocmd

---@type yay
yay = {}
