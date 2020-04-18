import DocsPage from '@hashicorp/react-docs-page'
import order from '../data/docs-navigation.js'
import { frontMatter } from '../pages/docs/**/*.mdx'
import { MDXProvider } from '@mdx-js/react'
import Placement from '../components/placement-table'
import Head from 'next/head'
import Link from 'next/link'

const DEFAULT_COMPONENTS = { Placement }

function DocsLayoutWrapper(pageMeta) {
  function DocsLayout(props) {
    return (
      <MDXProvider components={DEFAULT_COMPONENTS}>
        <DocsPage
          {...props}
          product="nomad"
          head={{
            is: Head,
            title: `${pageMeta.page_title} | Nomad by HashiCorp`,
            description: pageMeta.description,
            siteName: 'Nomad by HashiCorp'
          }}
          sidenav={{
            Link,
            category: 'docs',
            currentPage: props.path,
            data: frontMatter,
            order
          }}
          resourceURL={`https://github.com/hashicorp/nomad/blob/master/website/pages/${pageMeta.__resourcePath}`}
        />
      </MDXProvider>
    )
  }

  DocsLayout.getInitialProps = ({ asPath }) => ({ path: asPath })

  return DocsLayout
}

export default DocsLayoutWrapper
