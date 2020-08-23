import Head from 'next/head'
import Link from 'next/link'
import DocsPage from '@hashicorp/react-docs-page'
import { createMdxProvider } from '@hashicorp/nextjs-scripts/lib/providers/docs'
import { frontMatter as data } from '../pages/api-docs/**/*.mdx'
import Search from '../components/search'
import SearchProvider from '../components/search/provider'
import order from '../data/api-navigation.js'

const MDXProvider = createMdxProvider({ product: 'nomad' })

export default function ApiLayoutWrapper(pageMeta) {
  function ApiLayout(props) {
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
            category: 'api-docs',
            currentPage: props.path,
            data,
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

  ApiLayout.getInitialProps = ({ asPath }) => ({ path: asPath })

  return ApiLayout
}
