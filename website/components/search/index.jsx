import { useEffect, useState } from 'react'
import { Configure, InstantSearch } from 'react-instantsearch-dom'
import Hits from './hits'
import SearchBox from './search-box'
import { useSearch } from './provider'

export default function Search({ placeholder = 'Search' }) {
  const {
    indexName,
    initAlgoliaInsights,
    searchClient,
    searchQuery,
    setSearchQuery,
  } = useSearch()
  const [cancelSearch, setCancelSearch] = useState(false)

  useEffect(initAlgoliaInsights, [])

  function handleEscape() {
    setCancelSearch(true)
  }

  return (
    <div className="g-search">
      <InstantSearch indexName={indexName} searchClient={searchClient} refresh>
        <Configure distinct={1} hitsPerPage={25} clickAnalytics />
        <SearchBox
          {...{
            handleEscape,
            placeholder,
            searchQuery,
            setCancelSearch,
            setSearchQuery,
          }}
        />
        {searchQuery && !cancelSearch && (
          <Hits {...{ handleEscape, searchQuery, setCancelSearch }} />
        )}
      </InstantSearch>
    </div>
  )
}
