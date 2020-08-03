import Head from 'next/head'
import Link from 'next/link'
import DocsPage from '@hashicorp/react-docs-page'
import { createMdxProvider } from '@hashicorp/nextjs-scripts/lib/providers/docs'
import order from '../data/docs-navigation.js'
import { frontMatter } from '../pages/docs/**/*.mdx'
import Placement from '../components/placement-table'

const MDXProvider = createMdxProvider({
  product: 'nomad',
  additionalComponents: { Placement },
})

export default function DocsLayoutWrapper(pageMeta) {
  function DocsLayout(props) {
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
            category: 'docs',
            currentPage: props.path,
            data: frontMatter,
            order,
          }}
          resourceURL={`https://github.com/hashicorp/nomad/blob/master/website/pages/${pageMeta.__resourcePath}`}
        />
      </MDXProvider>
    )
  }

  DocsLayout.getInitialProps = ({ asPath }) => ({ path: asPath })

  return DocsLayout
}
