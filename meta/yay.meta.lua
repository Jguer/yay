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

---@alias yay.RebuildMode "no" | "yes" | "tree" | "all"

---@alias yay.Event "AURPreInstall" | "AURPostDownload" | "UpgradeSelect" | "PostInstall" | "SearchFilter"

-- Options: yay.opt

---@class yay.opt
--- Strings
---@field aururl string
---@field aurrpcurl string
---@field build_dir string
---@field editor string
---@field editor_flags string
---@field makepkg_bin string
---@field makepkg_conf string
---@field pacman_bin string
---@field pacman_conf string
---@field redownload string
---@field answer_clean string
---@field answer_diff string
---@field answer_edit string
---@field git_bin string
---@field gpg_bin string
---@field gpg_flags string
---@field mflags string
---@field sort_by string
---@field search_by string
---@field git_flags string
---@field remove_make string
---@field sudo_bin string
---@field sudo_flags string
--- Integers
---@field request_split_n integer
---@field completion_refresh_time integer
---@field max_concurrent_downloads integer
--- Booleans
---@field bottom_up boolean
---@field sudo_loop boolean
---@field devel boolean
---@field clean_after boolean
---@field keep_src boolean
---@field provides boolean
---@field pgp_fetch boolean
---@field clean_menu boolean
---@field diff_menu boolean
---@field edit_menu boolean
---@field combined_upgrade boolean
---@field use_ask boolean
---@field batch_install boolean
---@field single_line_results boolean
---@field separate_sources boolean
---@field debug boolean
---@field rpc boolean
---@field double_confirm boolean
--- Typed
---@field rebuild yay.RebuildMode

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

-- The yay global

---@class yay
---@field opt? yay.opt
---@field log? yay.log
---@field abort? fun(reason: string)

---@type yay
yay = {}

---@overload fun(event: "AURPreInstall", opts: yay.AURPreInstallOpts)
---@overload fun(event: "AURPostDownload", opts: yay.AURPostDownloadOpts)
---@overload fun(event: "UpgradeSelect", opts: yay.UpgradeSelectOpts)
---@overload fun(event: "PostInstall", opts: yay.PostInstallOpts)
---@overload fun(event: "SearchFilter", opts: yay.SearchFilterOpts)
---@diagnostic disable-next-line: inject-field
yay.create_autocmd = function(event, opts) end
