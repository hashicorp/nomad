set :base_url, "https://www.nomadproject.io/"

activate :hashicorp do |h|
  h.name        = "nomad"
  h.version     = "0.3.2"
  h.github_slug = "hashicorp/nomad"
end
