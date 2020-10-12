import Head from 'next/head'
import Link from 'next/link'
import { createMdxProvider } from '@hashicorp/nextjs-scripts/lib/providers/docs'
import DocsPage from '@hashicorp/react-docs-page'
import { SearchProvider } from '@hashicorp/react-search'
import { frontMatter as data } from '../pages/docs/**/*.mdx'
import Placement from '../components/placement-table'
import SearchBar from '../components/search-bar'
import order from '../data/docs-navigation.js'

const MDXProvider = createMdxProvider({
  product: 'nomad',
  additionalComponents: { Placement },
})

export default function DocsLayout({
  children,
  frontMatter,
  path,
  ...propsWithoutChildren
}) {
  return (
    <MDXProvider>
      <DocsPage
        {...propsWithoutChildren}
        product="nomad"
        head={{
          is: Head,
          title: `${frontMatter.page_title} | Nomad by HashiCorp`,
          description: frontMatter.description,
          siteName: 'Nomad by HashiCorp',
        }}
        sidenav={{
          Link,
          category: 'docs',
          currentPage: path,
          data,
          order,
          disableFilter: true,
        }}
        resourceURL={`https://github.com/hashicorp/nomad/blob/master/website/pages/${frontMatter.__resourcePath}`}
      >
        <SearchProvider>
          <SearchBar />
          {children}
        </SearchProvider>
      </DocsPage>
    </MDXProvider>
  )
}

DocsLayout.getInitialProps = ({ asPath }) => ({ path: asPath })
