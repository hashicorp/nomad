import Head from 'next/head'
import Link from 'next/link'
import hydrate from 'next-mdx-remote/hydrate'
import DocsPageComponent from '@hashicorp/react-docs-page'
import { SearchProvider } from '@hashicorp/react-search'
import SearchBar from '../../components/search-bar'
import order from '../../data/api-navigation'
import { generateStaticPaths, generateStaticProps } from '../../lib/docs-page'
import { MDX_COMPONENTS } from '../../lib/mdx-components'

export default function DocsPage({
  renderedContent,
  frontMatter,
  resourceUrl,
  url,
  sidenavData,
}) {
  const hydratedContent = hydrate(renderedContent, {
    components: MDX_COMPONENTS,
  })
  return (
    <DocsPageComponent
      product="nomad"
      head={{
        is: Head,
        title: `${frontMatter.page_title} | Nomad by HashiCorp`,
        description: frontMatter.description,
        siteName: 'Nomad by HashiCorp',
      }}
      sidenav={{
        Link,
        category: 'api-docs',
        currentPage: url,
        data: sidenavData,
        order,
        disableFilter: true,
      }}
      resourceURL={resourceUrl}
    >
      <SearchProvider>
        <SearchBar />
        {hydratedContent}
      </SearchProvider>
    </DocsPageComponent>
  )
}

export async function getStaticProps({ params }) {
  return generateStaticProps('api-docs', params)
}

export async function getStaticPaths() {
  return generateStaticPaths('api-docs')
}
