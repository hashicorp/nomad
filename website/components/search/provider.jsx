import { useState, createContext, useContext } from 'react'
import search from 'algoliasearch'
import aa from 'search-insights'

const SearchContext = createContext()

export const useSearch = () => {
  if (
    !process.env.NEXT_PUBLIC_ALGOLIA_APP_ID ||
    !process.env.NEXT_PUBLIC_ALGOLIA_SEARCH_ONLY_API_KEY ||
    !process.env.NEXT_PUBLIC_ALGOLIA_INDEX
  ) {
    throw new Error(`Missing all environment variables. Ensure the following are present in your environment:
- NEXT_PUBLIC_ALGOLIA_APP_ID
- NEXT_PUBLIC_ALGOLIA_SEARCH_ONLY_API_KEY
- NEXT_PUBLIC_ALGOLIA_INDEX
`)
  }
  return useContext(SearchContext)
}

const algoliaClient = search(
  process.env.NEXT_PUBLIC_ALGOLIA_APP_ID,
  process.env.NEXT_PUBLIC_ALGOLIA_SEARCH_ONLY_API_KEY
)

const searchClient = {
  search(requests) {
    return algoliaClient.search(
      requests.map((request) => {
        //  instantsearch fires an empty query on page load to ensure results are immediately available
        //  we exclude that result from our analytics to keep our clickthrough rate clean
        //  ref: https://www.algolia.com/doc/guides/getting-insights-and-analytics/search-analytics/out-of-the-box-analytics/how-to/how-to-remove-empty-search-from-analytics/
        if (!request.params.query || request.params.query.length === 0) {
          request.params.analytics = false
        }

        return request
      })
    )
  },
}

function initAlgoliaInsights() {
  aa('init', {
    appId: process.env.NEXT_PUBLIC_ALGOLIA_APP_ID,
    apiKey: process.env.NEXT_PUBLIC_ALGOLIA_SEARCH_ONLY_API_KEY,
  })
}

function logClick(hit) {
  return aa('clickedObjectIDsAfterSearch', {
    eventName: 'CLICK_HIT',
    index: process.env.NEXT_PUBLIC_ALGOLIA_INDEX,
    queryID: hit.__queryID,
    objectIDs: [hit.objectID],
    positions: [hit.__position],
  })
}

export default function SearchProvider({ children }) {
  const [searchQuery, setSearchQuery] = useState('')
  const [cancelSearch, setCancelSearch] = useState(false)

  return (
    <SearchContext.Provider
      value={{
        indexName: process.env.NEXT_PUBLIC_ALGOLIA_INDEX,
        initAlgoliaInsights,
        logClick,
        searchClient,
        searchQuery,
        setSearchQuery,
        cancelSearch,
        setCancelSearch,
      }}
    >
      {children}
    </SearchContext.Provider>
  )
}
