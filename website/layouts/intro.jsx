import Head from 'next/head'
import Link from 'next/link'
import DocsPage from '@hashicorp/react-docs-page'
import { SearchProvider } from '@hashicorp/react-search'
import { createMdxProvider } from '@hashicorp/nextjs-scripts/lib/providers/docs'
import { frontMatter as data } from '../pages/intro/**/*.mdx'
import SearchBar from '../components/search-bar'
import order from 'data/intro-navigation.js'

const MDXProvider = createMdxProvider({ product: 'nomad' })

export default function IntroLayout({
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
          category: 'intro',
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

IntroLayout.getInitialProps = ({ asPath }) => ({ path: asPath })
