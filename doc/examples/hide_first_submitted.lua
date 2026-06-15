yay.create_autocmd("SearchFilter", {
  desc = "hide AUR packages submitted in the last 14 days",
  callback = function(event)
    yay.log.info("hiding AUR packages submitted in the last 14 days")
    local out = {}
    local cutoff = os.time() - (14 * 24 * 60 * 60)
    for _, r in ipairs(event.data.results) do
      if r.source == "aur" and r.first_submitted ~= -1 and r.first_submitted >= cutoff then
        yay.log.debug("hiding newly submitted AUR package: ", r.name)
      else
        out[#out + 1] = { source = r.source, name = r.name }
      end
    end
    return out
  end,
})
