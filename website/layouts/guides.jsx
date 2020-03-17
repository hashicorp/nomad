import DocsPage from '@hashicorp/react-docs-page'
import order from '../data/guides-navigation.js'
import { frontMatter as data } from '../pages/guides/**/*.mdx'
import Head from 'next/head'
import Link from 'next/link'

function GuidesLayoutWrapper(pageMeta) {
  function GuidesLayout(props) {
    return (
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
          category: 'guides',
          currentPage: props.path,
          data,
          order
        }}
        resourceURL={`https://github.com/hashicorp/nomad/blob/master/website/pages/${pageMeta.__resourcePath}`}
      />
    )
  }

  GuidesLayout.getInitialProps = ({ asPath }) => ({ path: asPath })

  return GuidesLayout
}

export default GuidesLayoutWrapper
