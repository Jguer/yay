-- Exclude AUR upgrades younger than a minimum age (minage policy).
--
-- Enable from init.lua:
--   require("hooks.minage")
-- after copying this file to <config_dir>/hooks/minage.lua
--
-- What this hook does (via UpgradeSelect):
--   * Skips AUR/devel upgrades whose last_modified is newer than min_age_days.
--   * Keeps the installed version when local_version is set.
--   * Prunes pulled dependencies when an excluded upgrade requires it.
--
-- Limitations (hook API / yay data):
--   * Repo/sync upgrades have no last_modified in UpgradeSelect today; they are not filtered.
--   * Young packages in pulled_dependencies cannot be excluded (only warned); exclude only
--     accepts names from event.data.upgrades.
--   * No downgrade to an older pacman cache or AUR git commit.
--   * Does not run for yay -Qu (upgrade list only).
--
-- Hook design gaps compared to a core minage feature:
--   * UpgradeSelect only receives direct upgrade candidates, not a mutable graph.
--   * No way to pass a chosen older version or cache path back to yay.

local min_age_days = 3
-- false keeps yay's native exclusion menu and partial-upgrade warning (safer default).
local skip_menu = false

local SECONDS_PER_DAY = 24 * 60 * 60

local function is_aur_upgrade(repository)
  return repository == "aur" or repository == "devel"
end

local function package_too_young(last_modified, cutoff)
  return last_modified > 0 and last_modified >= cutoff
end

local function format_age_days(last_modified)
  local age = (os.time() - last_modified) / SECONDS_PER_DAY
  if age < 0 then
    return 0
  end
  return age
end

local function log_young_pkg(pkg, age_days, action)
  if pkg.local_version ~= "" then
    yay.log.warn(string.format(
      "minage: %s %s at %s (remote %s is only %.1f days old, minimum %d)",
      action, pkg.name, pkg.local_version, pkg.remote_version, age_days, min_age_days))
  else
    yay.log.warn(string.format(
      "minage: %s %s (%s, only %.1f days old, minimum %d)",
      action, pkg.name, pkg.remote_version, age_days, min_age_days))
  end
end

yay.create_autocmd("UpgradeSelect", {
  desc = "exclude AUR upgrades younger than min_age_days",
  callback = function(event)
    if min_age_days <= 0 then
      return { exclude = {}, skip_menu = false }
    end

    local upgrades = event.data and event.data.upgrades or {}
    local pulled = event.data and event.data.pulled_dependencies or {}
    local cutoff = os.time() - (min_age_days * SECONDS_PER_DAY)
    local exclude = {}

    for _, pkg in ipairs(upgrades) do
      if is_aur_upgrade(pkg.repository) and package_too_young(pkg.last_modified, cutoff) then
        local action = pkg.local_version ~= "" and "keeping" or "excluding"
        log_young_pkg(pkg, format_age_days(pkg.last_modified), action)
        exclude[#exclude + 1] = pkg.name
      end
    end

    for _, pkg in ipairs(pulled) do
      if is_aur_upgrade(pkg.repository) and package_too_young(pkg.last_modified, cutoff) then
        log_young_pkg(pkg, format_age_days(pkg.last_modified),
          "young pulled dependency (cannot exclude via hook, will still install)")
      end
    end

    if #exclude > 0 then
      yay.log.info(string.format("minage: excluded %d package(s)", #exclude))
    end

    return { exclude = exclude, skip_menu = skip_menu }
  end,
})
