#-------------------------------------------------------------------------
# Configure Middleman
#-------------------------------------------------------------------------

set :base_url, "https://www.nomadproject.io/"

activate :hashicorp do |h|
  h.version         = ENV["NOMAD_VERSION"]
  h.bintray_enabled = ENV["BINTRAY_ENABLED"]
  h.bintray_repo    = "mitchellh/nomad"
  h.bintray_user    = "mitchellh"
  h.bintray_key     = ENV["BINTRAY_API_KEY"]

  h.minify_javascript = false
end
