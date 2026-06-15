-- Example yay init.lua
--
-- This file is a complete template for yay.opt. Copy entries you need,
-- or keep all of them and tune values. Command-line flags still override
-- these values.

-- Strings
yay.opt.aururl = "https://aur.archlinux.org" -- Base AUR URL.
yay.opt.aurrpcurl = "https://aur.archlinux.org/rpc?" -- AUR RPC endpoint URL.
yay.opt.build_dir = os.getenv("HOME") .. "/.cache/yay" -- Build/cache directory for AUR packages.
yay.opt.editor = os.getenv("EDITOR") or os.getenv("VISUAL") or "vi" -- Editor command used for PKGBUILD edits; empty uses VISUAL/EDITOR.
yay.opt.editor_flags = "" -- Extra flags passed to the editor command.
yay.opt.makepkg_bin = "makepkg" -- makepkg executable (name in PATH or absolute path).
yay.opt.makepkg_conf = "" -- makepkg.conf path; empty uses default makepkg config.
yay.opt.pacman_bin = "pacman" -- pacman executable.
yay.opt.pacman_conf = "/etc/pacman.conf" -- pacman.conf file path.
yay.opt.redownload = "no" -- PKGBUILD download mode: "no" | "yes" | "all".
yay.opt.git_bin = "git" -- git executable.
yay.opt.gpg_bin = "gpg" -- gpg executable.
yay.opt.gpg_flags = "" -- Extra flags passed to gpg.
yay.opt.mflags = "" -- Extra flags passed to makepkg.
yay.opt.sort_by = "" -- AUR search sort field: "votes" | "popularity" | "name" | "base" | "submitted" | "modified" | "".
yay.opt.search_by = "name-desc" -- AUR search field: "name" | "name-desc" | "maintainer" | "depends" | "checkdepends" | "makedepends" | "optdepends" | "provides" | "conflicts" | "replaces" | "groups" | "keywords" | "comaintainers".
yay.opt.git_flags = "" -- Extra flags passed to git.
yay.opt.remove_make = "ask" -- Remove makedepends mode: "no" | "yes" | "ask" | "askyes".
yay.opt.sudo_bin = "sudo" -- Privilege elevation command.
yay.opt.sudo_flags = "" -- Extra flags passed to the sudo command.
yay.opt.rebuild = "no" -- Build mode: "no" | "yes" | "tree" | "all".

-- Integers
yay.opt.request_split_n = 150 -- Max packages per AUR RPC request (use values > 0).
yay.opt.completion_refresh_time = 7 -- Completion cache refresh days: -1 (never), 0 (always), >0 (every N days).
yay.opt.max_concurrent_downloads = 1 -- Parallel PKGBUILD source downloads; 0 uses CPU count.

-- Booleans
yay.opt.bottom_up = true -- Show AUR packages before repo packages in mixed results.
yay.opt.sudo_loop = false -- Keep sudo session alive in the background during long builds.
yay.opt.devel = false -- Check development/VCS packages on sysupgrade.
yay.opt.clean_after = false -- Remove untracked files after install.
yay.opt.keep_src = false -- Keep pkg/ and src/ after successful builds.
yay.opt.provides = true -- Resolve matching providers when dependencies are ambiguous.
yay.opt.pgp_fetch = true -- Prompt to import unknown PGP keys from validpgpkeys.
yay.opt.clean_menu = true -- Show pre-build clean menu.
yay.opt.diff_menu = true -- Show diff menu before building.
yay.opt.edit_menu = false -- Show PKGBUILD edit menu before building.
yay.opt.combined_upgrade = true -- Use combined repo+AUR upgrade flow on sysupgrade.
yay.opt.use_ask = false -- Use pacman's --ask to auto-confirm known conflicts.
yay.opt.batch_install = false -- Queue AUR package installs instead of installing each package immediately.
yay.opt.single_line_results = false -- Use single-line search result format.
yay.opt.separate_sources = true -- Separate query results by source (repo vs AUR).
yay.opt.debug = false -- Enable debug logging and local init.lua lookup convenience.
yay.opt.rpc = true -- Use AUR RPC for dependency/query operations.
yay.opt.double_confirm = true -- Ask for confirmation before and after builds during upgrades.

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
--       if pkg.installed then
--         yay.log.info(pkg.name .. " " .. pkg.version .. " installed (" .. pkg.source .. ")")
--       end
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
