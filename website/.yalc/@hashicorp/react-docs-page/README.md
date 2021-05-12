# DocsPage

The **DocsPage** component lets you create a Hashicorp branded docs page in NextJS projects using `next-mdx-remote`. This is a very highly abstracted component with slightly more involved usage since it renders an entire collection of pages.

## Example Usage

This component is intended to be used on an [optional catch-all route](https://nextjs.org/docs/routing/dynamic-routes#optional-catch-all-routes) page, like `pages/docs/[[...page]].mdx` - example source shown below:

```jsx
import DocsPage from '@hashicorp/react-docs-page'
// Imports below are only used server-side
import {
  generateStaticPaths,
  generateStaticProps,
} from '@hashicorp/react-docs-page/server'

//  Set up DocsPage settings
const BASE_ROUTE = 'docs'
const NAV_DATA = 'data/docs-nav-data.json'
const CONTENT_DIR = 'content/docs'
const PRODUCT = {
  name: 'Packer',
  slug: 'packer',
}

function DocsLayout(props) {
  return (
    <DocsPage baseRoute={BASE_ROUTE} product={PRODUCT} staticProps={props} />
  )
}

export async function getStaticPaths() {
  const paths = await generateStaticPaths({
    navDataFile: NAV_DATA,
    localContentDir: CONTENT_DIR,
  })
  return { paths, fallback: false }
}

export async function getStaticProps({ params }) {
  const props = await generateStaticProps({
    navDataFile: NAV_DATA,
    localContentDir: CONTENT_DIR,
    params,
    product: PRODUCT,
  })
  return { props }
}

export default DocsLayout
```

This may seem like a complex example, but there is a lot going on here. The component is taking care of an entire base-level route, including an index page and its potentially hundreds of sub-pages, while providing a minimal interface surface area.

In order for the search functionality to work properly, this component requires a `.env` file with the following keys filled in:

```
NEXT_PUBLIC_ALGOLIA_APP_ID
NEXT_PUBLIC_ALGOLIA_INDEX
NEXT_PUBLIC_ALGOLIA_SEARCH_ONLY_API_KEY
```

## Props

See [props.js](props.js) for more information on props.
