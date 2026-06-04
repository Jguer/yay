-- Example yay init.lua
--
-- Install as:
--   $XDG_CONFIG_HOME/yay/init.lua
-- or:
--   $HOME/.config/yay/init.lua

local function env(name, fallback)
  local value = yay.api.getenv(name)
  if value == nil or value == "" then
    return fallback
  end

  return value
end

yay.api.info("loading yay init.lua")

-- Normal configuration uses yay.opt.<key>.
yay.opt.editor = env("EDITOR", "nvim")
yay.opt.editor_flags = "-f"
yay.opt.build_dir = yay.api.expand("~/.cache/yay")
yay.opt.clean_menu = true
yay.opt.diff_menu = true
yay.opt.sort_by = "votes"
yay.opt.search_by = "name-desc"
yay.opt.request_split_n = 150

-- Prompt hooks can override the default static answer values.
yay.hook.on_prompt = function(name, default)
  if name == "clean" then
    return "None"
  end

  return default
end

-- Re-enable the old behavior of selecting AUR updates by update time instead
-- of only by version. This keeps the built-in version check, but also treats
-- unchanged-version AUR packages as upgrades when the AUR package metadata was
-- modified after the locally installed package was built.
yay.hook.should_include_aur_update = function(pkg)
  if pkg.default_include then
    return true
  end

  if pkg.repository ~= "aur" then
    return false
  end

  if pkg.local_version ~= pkg.remote_version then
    return false
  end

  if pkg.local_build_date == 0 then
    return false
  end

  return pkg.remote_last_modified > pkg.local_build_date
end

-- External providers live entirely in Lua. A provider can expose:
--   yay.provider.<name>.search(terms) -> { { name=..., repository=..., ... }, ... }
--   yay.provider.<name>.install(items)
--   yay.provider.<name>.list() -> { { name=..., repository=..., ... }, ... }
--   yay.provider.<name>.upgrade(items)
--
-- This Homebrew example adds search/install support for the yogurt menu and
-- uses `brew outdated --json=v2` for upgrade discovery.
yay.provider.homebrew = {
  search = function(terms)
    local args = { "brew", "search", "--formula", "--cask" }
    for _, term in ipairs(terms) do
      table.insert(args, term)
    end

    local stdout, stderr, code = yay.api.capture(args)
    if code ~= 0 then
      yay.api.warn("homebrew search skipped: " .. stderr)
      return {}
    end

    local results = {}
    for name in stdout:gmatch("[^\r\n]+") do
      table.insert(results, {
        name = name,
        repository = "homebrew",
        description = "Homebrew package",
      })
    end

    return results
  end,

  install = function(items)
    local args = { "brew", "install" }
    for _, item in ipairs(items) do
      table.insert(args, item.name)
    end
    if yay.api.run(args) ~= 0 then
      error("homebrew install failed")
    end
  end,

  list = function()
    local stdout, stderr, code = yay.api.capture({ "brew", "outdated", "--json=v2" })
    if code ~= 0 then
      yay.api.warn("homebrew provider skipped: " .. stderr)
      return {}
    end

    local decoded = yay.api.json_decode(stdout)
    local upgrades = {}

    for _, item in ipairs(decoded.formulae or {}) do
      table.insert(upgrades, {
        name = item.name,
        repository = "homebrew",
        base = "formula",
        local_version = (item.installed_versions and item.installed_versions[1]) or "",
        remote_version = item.current_version or "",
      })
    end

    for _, item in ipairs(decoded.casks or {}) do
      table.insert(upgrades, {
        name = item.name,
        repository = "homebrew",
        base = "cask",
        local_version = (item.installed_versions and item.installed_versions[1]) or "",
        remote_version = item.current_version or "",
        extra = "cask",
      })
    end

    return upgrades
  end,

  upgrade = function(items)
    local formulae = {}
    local casks = {}

    for _, item in ipairs(items) do
      if item.base == "cask" then
        table.insert(casks, item.name)
      else
        table.insert(formulae, item.name)
      end
    end

    if #formulae > 0 then
      local args = { "brew", "upgrade" }
      for _, name in ipairs(formulae) do
        table.insert(args, name)
      end
      if yay.api.run(args) ~= 0 then
        error("homebrew formula upgrade failed")
      end
    end

    if #casks > 0 then
      local args = { "brew", "upgrade", "--cask" }
      for _, name in ipairs(casks) do
        table.insert(args, name)
      end
      if yay.api.run(args) ~= 0 then
        error("homebrew cask upgrade failed")
      end
    end
  end,
}