-- Reproduce yay.opt.single_line_results via RenderAUR/RenderSync.
--
-- yay.opt.single_line_results only changes one thing in the built-in
-- renderer: the description is joined onto the result line with a tab
-- instead of being printed on its own indented line below. These two hooks
-- rebuild that same layout (plus the bold/color accents) from scratch, so
-- you have a starting point for further tweaks (drop fields, change colors,
-- add links, etc.). yay does not expose a color/text helper API to Lua, so
-- every ANSI escape below is built by hand with plain Lua string functions.

local ESC = string.char(27)
local SEPARATOR = "\t" -- the one change that makes a result single-line

local MINUTE, HOUR, DAY = 60, 3600, 24 * 3600

local function sgr(code, s)
  return ESC .. "[" .. code .. "m" .. s .. ESC .. "[0m"
end

local function bold(s) return sgr("1", s) end
local function red(s) return sgr("31", s) end
local function green(s) return sgr("32", s) end
local function yellow(s) return sgr("33", s) end
local function cyan(s) return sgr("36", s) end

-- A given repo/package name always renders in the same one of six colors,
-- same hashing idea as yay's internal ColorHash.
local function color_hash(name)
  local hash = 0
  for i = 1, #name do
    hash = (string.byte(name, i) + hash * 32 + hash) % 2147483648
  end
  return sgr(tostring(hash % 6 + 31), name)
end

-- Common "repo/name version" prefix shared by AUR and sync result lines.
local function header(repo, name, version)
  return bold(color_hash(repo)) .. "/" .. bold(name) .. " " .. cyan(version)
end

-- Appends " tag" to line, unless tag is empty/nil/false.
local function append(line, tag)
  if not tag or tag == "" then
    return line
  end
  return line .. " " .. tag
end

-- "[Xm]"/"[Xh]"/"[Xd]" age badge, colored red/yellow/cyan the newer it is.
local function age_tag(last_modified)
  if not last_modified or last_modified == 0 then
    return ""
  end

  local age = os.time() - last_modified
  if age < HOUR then
    return bold(red("[" .. math.floor(age / MINUTE) .. "m]"))
  elseif age < DAY then
    return bold(red("[" .. math.floor(age / HOUR) .. "h]"))
  elseif age < 7 * DAY then
    return bold(yellow("[" .. math.floor(age / DAY) .. "d]"))
  end
  return cyan("[" .. math.floor(age / DAY) .. "d]")
end

local function installed_tag(version, local_version)
  if local_version == "" then
    return ""
  elseif local_version == version then
    return bold(green("(Installed)"))
  end
  return bold(green("(Installed: " .. local_version .. ")"))
end

yay.create_autocmd("RenderAUR", {
  desc = "single-line, colored AUR search results (mirrors single_line_results)",
  callback = function(event)
    local d = event.data

    local line = header("aur", d.name, d.version) ..
      bold(" (+" .. d.votes) .. " " .. bold(string.format("%.2f", d.popularity) .. ")")

    line = append(line, age_tag(d.last_modified))
    line = append(line, d.maintainer == "" and bold(red("(Orphaned)")))
    line = append(line, d.out_of_date ~= 0 and bold(red("(Out-of-date)")))
    line = append(line, installed_tag(d.version, d.local_version))

    return line .. SEPARATOR .. d.description
  end,
})

yay.create_autocmd("RenderSync", {
  desc = "single-line, colored sync search results (mirrors single_line_results)",
  callback = function(event)
    local d = event.data

    local line = header(d.repository, d.name, d.version)
    line = append(line, #d.groups > 0 and table.concat(d.groups, " "))
    line = append(line, installed_tag(d.version, d.local_version))

    return line .. SEPARATOR .. d.description
  end,
})
