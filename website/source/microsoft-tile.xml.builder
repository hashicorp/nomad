---
layout: false
noindex: true
---

xml.instruct!
xml.browserconfig do
  xml.msapplication do
    xml.tile do
      xml.square70x70logo src: image_path("favicons/mstile-70x70.png")
      xml.square150x150logo src: image_path("favicons/mstile-150x150.png")
      xml.square310x310logo src: image_path("favicons/mstile-310x310.png")
      xml.square310x150logo src: image_path("favicons/mstile-310x150.png")
      xml.TileColor "#25BA81"
    end
  end
end
