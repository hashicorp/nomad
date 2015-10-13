#-------------------------------------------------------------------------
# Configure Middleman
#-------------------------------------------------------------------------

helpers do
  def livestream_active?
    # Must set key for date
    ENV["LIVESTREAM_ACTIVE"].present?
  end
end

set :base_url, "https://www.nomadproject.io/"

activate :hashicorp do |h|
  h.version         = ENV["NOMAD_VERSION"]
  h.bintray_enabled = ENV["BINTRAY_ENABLED"] == "1"
  h.bintray_repo    = "mitchellh/nomad"
  h.bintray_user    = "mitchellh"
  h.bintray_key     = ENV["BINTRAY_API_KEY"]
  h.github_slug     = "hashicorp/nomad"

  h.minify_javascript = false
end
