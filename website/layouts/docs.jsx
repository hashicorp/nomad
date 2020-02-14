import DocsPage, { getInitialProps } from '../components/docs-page'
import orderData from '../data/docs-navigation.js'
import { frontMatter } from '../pages/docs/**/*.mdx'
import { MDXProvider } from '@mdx-js/react'
import Placement from '../components/placement-table'

const DEFAULT_COMPONENTS = { Placement }

function DocsLayoutWrapper(pageMeta) {
  function DocsLayout(props) {
    return (
      <MDXProvider components={DEFAULT_COMPONENTS}>
        <DocsPage
          {...props}
          orderData={orderData}
          frontMatter={frontMatter}
          category="docs"
          pageMeta={pageMeta}
        />
      </MDXProvider>
    )
  }

  DocsLayout.getInitialProps = getInitialProps

  return DocsLayout
}

export default DocsLayoutWrapper
