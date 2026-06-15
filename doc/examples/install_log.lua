local log_path = (os.getenv("XDG_STATE_HOME") or (os.getenv("HOME") .. "/.local/state")) .. "/yay/install.log"
local log_dir  = log_path:match("^(.+)/[^/]+$")

yay.create_autocmd("PostInstall", {
  desc = "append every installed/upgraded package to a persistent log",
  callback = function(event)
    yay.log.info("install_log: writing to ", log_path)   
    os.execute("mkdir -p " .. log_dir)
    local f, err = io.open(log_path, "a")
    if not f then
      yay.log.warn("install_log: cannot open log file: ", err)
      return
    end

    local ts = os.date("%Y-%m-%dT%H:%M:%S")

    for _, pkg in ipairs(event.data.packages) do
      if pkg.installed then
        local action = pkg.upgrade and "upgrade" or "install"
        local version_change
        if pkg.upgrade then
          version_change = pkg.local_version .. " -> " .. pkg.version
        else
          version_change = pkg.version
        end

        local flags = pkg.devel and " devel" or ""
        f:write(string.format("%s  %-9s %-7s %-14s %-12s %s%s\n",
          ts, action, pkg.source, pkg.reason, pkg.name, version_change, flags))
      end
    end

    f:close()
  end,
})
