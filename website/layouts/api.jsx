import DocsPage from '@hashicorp/react-docs-page'
import order from 'data/api-navigation.js'
import { frontMatter as data } from '../pages/api-docs/**/*.mdx'
import Head from 'next/head'
import Link from 'next/link'
import { createMdxProvider } from '@hashicorp/nextjs-scripts/lib/providers/docs'

const MDXProvider = createMdxProvider({ product: 'nomad' })

export default function ApiLayoutWrapper(pageMeta) {
  function ApiLayout(props) {
    return (
      <MDXProvider>
        <DocsPage
          {...props}
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
          }}
          resourceURL={`https://github.com/hashicorp/nomad/blob/master/website/pages/${pageMeta.__resourcePath}`}
        />
      </MDXProvider>
    )
  }

  ApiLayout.getInitialProps = ({ asPath }) => ({ path: asPath })

  return ApiLayout
}
