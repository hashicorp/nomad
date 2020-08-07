import DocsPage from '@hashicorp/react-docs-page'
import order from 'data/intro-navigation.js'
import { frontMatter as data } from '../pages/intro/**/*.mdx'
import Head from 'next/head'
import Link from 'next/link'
import { createMdxProvider } from '@hashicorp/nextjs-scripts/lib/providers/docs'
import Search from '../components/search'
import SearchProvider from '../components/search/provider'

const MDXProvider = createMdxProvider({ product: 'nomad' })

export default function IntroLayoutWrapper(pageMeta) {
  function IntroLayout(props) {
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
            category: 'intro',
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

  IntroLayout.getInitialProps = ({ asPath }) => ({ path: asPath })

  return IntroLayout
}
