# HashiCorp Search Component

### Properties

`placeholder` (Optional) `string`. Default: `Search`

```jsx
import Search from '../path/to/components/search'

function Search() {
  return <Search placeholder="Search documentation" />
}
```

### Usage

We rely on the presence of the following environment variables present on the client:

```text
NEXT_PUBLIC_ALGOLIA_APP_ID
NEXT_PUBLIC_ALGOLIA_SEARCH_ONLY_API_KEY
NEXT_PUBLIC_ALGOLIA_INDEX
```

To use the primary `<Search />` component, ensure it exists as a child of the `<SearchProvider />` component. For example:

**App.jsx**

```jsx
import Search from 'components/search'
import SearchProvider from 'components/search/provider'

function App() {
  return (
    <>
      <SearchProvider>
        <Search />
        <ComponentA>
        <ComponentB>
      </SearchProvider>
      <ComponentC__WithoutSearchContext>
    </>
  )
}
```

Any child component of `<SearchProvider />` can utilize the provided `useSearch()` hook and access search-specific information. For example:

```jsx
import { useSearch } from 'components/search/provider'

function ComponentA() {
  const { searchQuery } = useSearch()

  return <code>Search query: {searchQuery}</code>
}
```

### useSearch()

`useSearch()` exposes the following values:

- [ **Public** ] `activeOnMobile` (`boolean`) - Indicates whether the mobile toggle has been activated
- [ _Internal_ ] `indexName` (`string`) - The name of the Algolia index that search is performed upon
- [ _Internal_ ] `initAlgoliaInsights` (`function`) - Required to initialize Algolia
- [ _Internal_ ] `logClick` (`function`) - Fires an analytics event via the `search-insights` package
- [ _Internal_ ] `searchClient` (`object`) - Initialized Algolia client
- [ **Public** ] `searchQuery` (`string`) - Current search query
- [ _Internal_ ] `setActiveOnMobile` (`function`) - Setter function that toggles the state of mobile activation
- [ _Internal_ ] `setSearchQuery` (`function`) - Setter function that updates the search query
