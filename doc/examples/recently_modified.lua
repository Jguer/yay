yay.create_autocmd("UpgradeSelect", {
  desc = "skip recently modified AUR upgrades",
  callback = function(event)
    yay.log.info("pre-excluding AUR packages modified in the last 3 days")
    local exclude = {}
    local recent_cutoff = os.time() - (3 * 24 * 60 * 60)
    for _, pkg in ipairs(event.data.upgrades) do
      if pkg.repository == "aur" and pkg.last_modified >= recent_cutoff then
        yay.log.warn("pre-excluding recently modified AUR package: ", pkg.name)
        table.insert(exclude, pkg.name)
      end
    end

    return { exclude = exclude, skip_menu = false }
  end,
})