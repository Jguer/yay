-- Warn when an AUR package's maintainer changes between upgrades.
--
-- The known maintainer for each package is stored in a plain text cache file
-- inside the yay cache directory (build_dir). On the first upgrade for a
-- package the current maintainer is recorded without any warning. On
-- subsequent upgrades:
--   * same maintainer  → debug "match correct"
--   * different maintainer → error "new maintainer, double check build files"
--
-- The cache is updated whenever a new or changed maintainer is seen.
--
-- Cache file location: <build_dir>/maintainer_cache
-- Format: one "pkgname=maintainer" entry per line.

local cache_file = yay.opt.build_dir .. "/maintainer_cache"

local function load_cache()
  local cache = {}
  local f = io.open(cache_file, "r")
  if not f then return cache end
  for line in f:lines() do
    local name, maintainer = line:match("^([^=]+)=(.*)$")
    if name then
      cache[name] = maintainer
    end
  end
  f:close()
  return cache
end

local function save_cache(cache)
  local f = assert(io.open(cache_file, "w"))
  for name, maintainer in pairs(cache) do
    f:write(name .. "=" .. maintainer .. "\n")
  end
  f:close()
end

yay.create_autocmd("UpgradeSelect", {
  desc = "warn on AUR maintainer changes",
  callback = function(event)
    yay.log.info("checking for AUR maintainer changes")
    local cache = load_cache()
    local dirty = false

    for _, pkg in ipairs(event.data.upgrades) do
      if pkg.repository == "aur" and pkg.maintainer ~= "" then
        local cached = cache[pkg.name]
        if cached == nil then
          -- First time seeing this package: seed the cache silently.
          cache[pkg.name] = pkg.maintainer
          dirty = true
        elseif cached == pkg.maintainer then
          yay.log.debug("match correct: " .. pkg.name .. " " .. pkg.maintainer)
        else
          yay.log.error("new maintainer, double check build files: ", pkg.name,
            "(was: " .. cached .. ", now: " .. pkg.maintainer .. ")")
          cache[pkg.name] = pkg.maintainer
          dirty = true
        end
      end
    end

    if dirty then
      yay.log.info("saving maintainer cache:", cache_file)
      save_cache(cache)
    end

    return { exclude = {}, skip_menu = false }
  end,
})
