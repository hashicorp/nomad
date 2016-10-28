set :base_url, "https://www.nomadproject.io/"

activate :hashicorp do |h|
  h.name        = "nomad"
  h.version     = "0.4.1"
  h.github_slug = "hashicorp/nomad"
end

helpers do
  # Get the title for the page.
  #
  # @param [Middleman::Page] page
  #
  # @return [String]
  def title_for(page)
    if page && page.data.page_title
      return "#{page.data.page_title} - Nomad by HashiCorp"
    end

     "Nomad by HashiCorp"
   end

  # Get the description for the page
  #
  # @param [Middleman::Page] page
  #
  # @return [String]
  def description_for(page)
    return escape_html(page.data.description || "")
  end

  # Returns the id for this page.
  # @return [String]
  def body_id_for(page)
    if name = page.data.sidebar_current && !name.blank?
      return "page-#{name.strip}"
    end
    return "page-home"
  end

  # Returns the list of classes for this page.
  # @return [String]
  def body_classes_for(page)
    classes = []

    if page && page.data.layout
      classes << "layout-#{page.data.layout}"
    end

    classes << "-displaying-bnr"

    return classes.join(" ")
  end
end
