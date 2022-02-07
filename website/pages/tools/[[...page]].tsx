import { productName, productSlug } from 'data/metadata'
import DocsPage from '@hashicorp/react-docs-page'
import { getStaticGenerationFunctions } from '@hashicorp/react-docs-page/server'

const NAV_DATA_FILE = 'data/tools-nav-data.json'
const CONTENT_DIR = 'content/tools'
const basePath = 'tools'

export default function DocsLayout(props) {
  return (
    <DocsPage
      product={{ name: productName, slug: productSlug }}
      baseRoute={basePath}
      staticProps={props}
    />
  )
}

const { getStaticPaths, getStaticProps } = getStaticGenerationFunctions({
  strategy: 'fs',
  localContentDir: CONTENT_DIR,
  navDataFile: NAV_DATA_FILE,
  product: productSlug,
})

export { getStaticPaths, getStaticProps }
