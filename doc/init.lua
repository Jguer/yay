-- Example yay init.lua
--
-- This file is a complete template for yay.opt. Copy entries you need,
-- or keep all of them and tune values. Command-line flags still override
-- these values.

yay.opt.aururl = "https://aur.archlinux.org"
yay.opt.aurrpcurl = ""
yay.opt.build_dir = os.getenv("HOME") .. "/.cache/yay"
yay.opt.editor = os.getenv("EDITOR") or os.getenv("VISUAL") or "vi"
yay.opt.editor_flags = ""
yay.opt.makepkg_bin = "makepkg"
yay.opt.makepkg_conf = ""
yay.opt.pacman_bin = "pacman"
yay.opt.pacman_conf = "/etc/pacman.conf"
yay.opt.redownload = "no"
yay.opt.git_bin = "git"
yay.opt.gpg_bin = "gpg"
yay.opt.gpg_flags = ""
yay.opt.mflags = ""
yay.opt.sort_by = ""
yay.opt.search_by = "name-desc"
yay.opt.git_flags = ""
yay.opt.remove_make = "ask"
yay.opt.sudo_bin = "sudo"
yay.opt.sudo_flags = ""
yay.opt.rebuild = "no"
yay.opt.answer_clean = ""
yay.opt.answer_diff = ""
yay.opt.answer_edit = ""

yay.opt.request_split_n = 150
yay.opt.completion_refresh_time = 7
yay.opt.max_concurrent_downloads = 1

yay.opt.bottom_up = true
yay.opt.sudo_loop = false
yay.opt.devel = false
yay.opt.clean_after = false
yay.opt.keep_src = false
yay.opt.provides = true
yay.opt.pgp_fetch = true
yay.opt.clean_menu = true
yay.opt.diff_menu = true
yay.opt.edit_menu = false
yay.opt.combined_upgrade = true
yay.opt.use_ask = false
yay.opt.batch_install = false
yay.opt.single_line_results = false
yay.opt.separate_sources = true
yay.opt.debug = false
yay.opt.rpc = true
yay.opt.double_confirm = true

-- Hooks
-- Run Lua before yay prints the upgrade exclusion menu. Return package names
-- from event.data.upgrades to pre-exclude them. Set skip_menu = false, or omit
-- it, to show the native menu after these exclusions are applied.
--
-- yay.create_autocmd("UpgradeSelect", {
--   desc = "skip recently modified AUR upgrades",
--   callback = function(event)
--     local exclude = {}
--     local recent_cutoff = os.time() - (3 * 24 * 60 * 60)
--     for _, pkg in ipairs(event.data.upgrades) do
--       if pkg.repository == "aur" and pkg.last_modified >= recent_cutoff then
--         yay.log.warn("pre-excluding recently modified AUR package:", pkg.name)
--         table.insert(exclude, pkg.name)
--       end
--     end
--
--     return { exclude = exclude, skip_menu = false }
--   end,
-- })
--
-- Run Lua after AUR PKGBUILD repos are downloaded/merged and before the
-- clean/diff/edit menus or source downloads.
--
-- yay.create_autocmd("AURPreInstall", {
--   desc = "inspect or modify AUR package files",
--   callback = function(event)
--     if event.data.pkgbuild:match("forbidden.example") then
--       yay.log.warn(event.match .. ": forbidden source URL")
--       yay.abort(event.match .. ": forbidden source URL")
--     end
--
--     -- File edits are picked up by later menus and build steps.
--     -- local path = event.data.pkgbuild_path
--     -- local f = assert(io.open(path, "a"))
--     -- f:write("\n# edited by yay init.lua\n")
--     -- f:close()
--   end,
-- })
--
-- Run Lua after yay downloads/verifies package sources and before builds or
-- installs. AURPostDownload receives the same payload shape as AURPreInstall.
--
-- yay.create_autocmd("AURPostDownload", {
--   desc = "block forbidden source URLs after download",
--   callback = function(event)
--     if event.data.pkgbuild:match("forbidden.example") then
--       yay.abort(event.match .. ": forbidden source URL")
--     end
--   end,
-- })
--
-- Run Lua once after a successful install/upgrade transaction (skipped on
-- --downloadonly). The callback is fire-and-forget; returning has no effect.
--
-- yay.create_autocmd("PostInstall", {
--   desc = "log every package yay installed",
--   callback = function(event)
--     for _, pkg in ipairs(event.data.packages) do
--       yay.log.info(pkg.name .. " " .. pkg.version .. " (" .. pkg.source .. ")")
--     end
--   end,
-- })
--
-- Run Lua during -Ss / -S number menu after ranking, before display. Return
-- an ordered array of {source=, name=} to filter/reorder; nil = unchanged.
--
-- yay.create_autocmd("SearchFilter", {
--   desc = "show only AUR results",
--   callback = function(event)
--     local out = {}
--     for _, r in ipairs(event.data.results) do
--       if r.source == "aur" then
--         out[#out + 1] = { source = r.source, name = r.name }
--       end
--     end
--     return out
--   end,
-- })
