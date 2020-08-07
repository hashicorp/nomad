import Head from 'next/head'
import Link from 'next/link'
import DocsPage from '@hashicorp/react-docs-page'
import { createMdxProvider } from '@hashicorp/nextjs-scripts/lib/providers/docs'
import order from '../data/docs-navigation.js'
import { frontMatter } from '../pages/docs/**/*.mdx'
import Placement from '../components/placement-table'
import Search from '../components/search'
import SearchProvider from '../components/search/provider'

const MDXProvider = createMdxProvider({
  product: 'nomad',
  additionalComponents: { Placement },
})

export default function DocsLayoutWrapper(pageMeta) {
  function DocsLayout(props) {
    const { children, ...propsWithoutChildren } = props
    return (
      <MDXProvider>
        <DocsPage
          {...propsWithoutChildren}
          product="nomad"
          head={{
            is: Head,
            title: `${pageMeta.page_title} | Nomad by HashiCorp`,
            description: pageMeta.description,
            siteName: 'Nomad by HashiCorp',
          }}
          sidenav={{
            Link,
            category: 'docs',
            currentPage: props.path,
            data: frontMatter,
            order,
            disableFilter: true,
          }}
          resourceURL={`https://github.com/hashicorp/nomad/blob/master/website/pages/${pageMeta.__resourcePath}`}
        >
          <SearchProvider>
            <Search placeholder="Search Nomad documentation" />
            {children}
          </SearchProvider>
        </DocsPage>
      </MDXProvider>
    )
  }

  DocsLayout.getInitialProps = ({ asPath }) => ({ path: asPath })

  return DocsLayout
}
