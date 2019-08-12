set :base_url, "https://www.nomadproject.io/"

activate :hashicorp do |h|
  h.name        = "nomad"
  h.version     = "0.9.4"
  h.github_slug = "hashicorp/nomad"
end

helpers do
  # Returns a segment tracking ID such that local development is not
  # tracked to production systems.
  def segmentId()
    if (ENV['ENV'] == 'production')
      'qW11yxgipKMsKFKQUCpTVgQUYftYsJj0'
    else
      '0EXTgkNx0Ydje2PGXVbRhpKKoe5wtzcE'
    end
  end

  # Returns the FQDN of the image URL.
  #
  # @param [String] path
  #
  # @return [String]
  def image_url(path)
    File.join(base_url, image_path(path))
  end

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
    description = (page.data.description || "")
      .gsub('"', '')
      .gsub(/\n+/, ' ')
      .squeeze(' ')

    return escape_html(description)
  end

  # This helps by setting the "active" class for sidebar nav elements
  # if the YAML frontmatter matches the expected value.
  def sidebar_current(expected)
    current = current_page.data.sidebar_current || ""
    if current.start_with?(expected)
      return " class=\"active\""
    else
      return ""
    end
  end

  # Returns a <ul> with links to API subheaders
  def api_toc(path)
    html_toc = Redcarpet::Markdown.new(Redcarpet::Render::HTML_TOC.new(nesting_level: 2..2))
    file = ::File.read(path)

    # remove YAML frontmatter
    file = file.gsub(/^(---\s*\n.*?\n?)^(---\s*$\n?)/m,'')

    rendered = html_toc.render file
    rendered.gsub("<ul", "<ul class='api-nav'")
  end

  # Inserts a TOC before the first subhead
  def insert_api_toc(html)
    doc = Nokogiri::HTML::DocumentFragment.parse(html)

    h2_count = doc.css("h2").length

    if h2_count > 1
      first_h2 = doc.at_css "h2"
      first_h2.add_previous_sibling(api_toc("source/#{current_page.path}.md"))

      doc.to_html
    else
      html
    end
  end

  # Returns the id for this page.
  # @return [String]
  def body_id_for(page)
    if !(name = page.data.sidebar_current).blank?
      return "page-#{name.strip}"
    end
    if page.url == "/" || page.url == "/index.html"
      return "page-home"
    end
    if !(title = page.data.page_title).blank?
      return title
        .downcase
        .gsub('"', '')
        .gsub(/[^\w]+/, '-')
        .gsub(/_+/, '-')
        .squeeze('-')
        .squeeze(' ')
    end
    return ""
  end

  # Returns the list of classes for this page.
  # @return [String]
  def body_classes_for(page)
    classes = []

    if !(layout = page.data.layout).blank?
      classes << "layout-#{page.data.layout}"
    end

    if !(title = page.data.page_title).blank?
      title = title
        .downcase
        .gsub('"', '')
        .gsub(/[^\w]+/, '-')
        .gsub(/_+/, '-')
        .squeeze('-')
        .squeeze(' ')
      classes << "page-#{title}"
    end

    return classes.join(" ")
  end
end
