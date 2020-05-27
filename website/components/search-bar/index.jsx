import { useEffect, useState } from 'react'
import {
  InstantSearch,
  SearchBox,
  connectHits,
  Configure,
} from 'react-instantsearch-dom'
import InlineSvg from '@hashicorp/react-inline-svg'
import SearchIcon from './img/search.svg?include'
import Hit from './hit'
import SearchLegend from './search-legend'
import { searchClient, initAlgoliaInsights } from '../../lib/algolia'

const ALGOLIA_SELECTORS = {
  input: '.ais-SearchBox-input',
}

export default function SearchBar({ mobileInputActive, onToggleMobileInput }) {
  const [searchQuery, setInputState] = useState('')
  const [cancelSearch, setCancelSearch] = useState(false)

  useEffect(() => {
    initAlgoliaInsights()
  }, [])

  useEffect(() => {
    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [])

  function onKeyDown(e) {
    const inputElements = ['INPUT', 'TEXTAREA']

    // Listen for `/` key presses, while ensuring other text inputs
    // are still able to use `/` key in their respective text entries
    if (e.keyCode === 191 && !inputElements.includes(e.target.tagName)) {
      e.preventDefault()
      focusInputField()
    }
  }

  function focusInputField() {
    document.querySelector(ALGOLIA_SELECTORS['input']).focus()
  }

  function handleSearchboxKeyDown(e) {
    if (!searchQuery) return
    // Regain active search if previously cancelled
    setCancelSearch(false)
    if (e.keyCode === 27) return handleEscape()
  }

  // Update inputState like a normal controlled input
  function handleSearchboxChange(e) {
    return setInputState(e.target.value)
  }

  // Clear inputState & reset tabIndex to null
  function handleSearchboxReset() {
    return setInputState('')
  }

  function handleEscape() {
    setCancelSearch(true)
  }

  return (
    <>
      <div className="g-search-bar">
        <InstantSearch
          indexName={process.env.ALGOLIA_INDEX}
          searchClient={searchClient}
          refresh
        >
          <Configure distinct={1} hitsPerPage={25} clickAnalytics />
          <SearchBox
            onChange={handleSearchboxChange}
            reset={
              <img src={require('./img/search-x.svg')} alt="Reset search" />
            }
            translations={{
              placeholder: `Search nomad documentation`,
            }}
            onReset={handleSearchboxReset}
            onKeyDown={handleSearchboxKeyDown}
            slowLoadingIndicator
          />

          {/* Keybound '/' to Search Element */}
          <img
            className="slash-search-toggle"
            src={require('./img/slash-search-toggle.svg')}
            alt="Type '/' to Search"
          />

          {searchQuery && !cancelSearch && (
            <AugmentedHits
              {...{ handleEscape, searchQuery, setCancelSearch }}
            />
          )}
        </InstantSearch>
      </div>

      {!mobileInputActive && (
        <button
          className="mobile-input-trigger"
          onClick={() => onToggleMobileInput(true)}
          aria-label="Show Search Bar"
        >
          <InlineSvg src={SearchIcon} aria-hidden={true} />
        </button>
      )}
    </>
  )
}

const Hits = ({ hits, handleEscape, searchQuery, setCancelSearch }) => {
  const [hitsTabIndex, setHitsTabIndex] = useState(null)
  const [selectedHit, setSelectedHit] = useState(null)

  useEffect(() => {
    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [hitsTabIndex])

  // Watch for changes to tabIndex and load up the proper active element
  useEffect(() => {
    // If we have a tabIndex, load the active Hit's DOM ref in case it should be clicked via `Enter` key
    if (hitsTabIndex) {
      let el = document.querySelector('.hit-link-wrapper.active')
      setSelectedHit(el)
      scrollToActive(el)
    }
  }, [hitsTabIndex])

  function onKeyDown(e) {
    switch (e.keyCode) {
      // [Enter]
      case 13:
        return handleEnter()
      // [Escape]
      case 27:
        return handleEscape()
      // [ArrowDown]
      case 40:
        if (!hitsTabIndex) {
          setHitsTabIndex(0)
          scrollToActive()
        }
        return handleArrowDown()
      // [ArrowUp]
      case 38:
        e.preventDefault() // prevent cursor from moving to start of search input
        return handleArrowUp()
    }
  }

  function handleEnter() {
    if (!selectedHit) return
    selectedHit.click()
  }

  function handleArrowUp() {
    decrementTabIndex()
  }

  function handleArrowDown() {
    incrementTabIndex()
  }

  function incrementTabIndex() {
    let startIndex = hitsTabIndex || 0
    const nextIndex = startIndex + 1
    if (nextIndex > hits.length) return setHitsTabIndex(1)
    setHitsTabIndex(nextIndex)
  }

  function decrementTabIndex() {
    let startIndex = hitsTabIndex || 0
    const nextIndex = startIndex - 1
    if (nextIndex < 1) return setHitsTabIndex(hits.length)
    setHitsTabIndex(nextIndex)
  }

  function scrollToActive(el) {
    if (!el) return
    el.scrollIntoView({
      behavior: 'smooth',
      block: 'center',
    })
  }

  return (
    <>
      {hits.length === 0 ? (
        <div className="no-results-text">
          <span className="title">{`No results for ${searchQuery}...`}</span>
          <span className="message">
            Search tips: some terms require an exact match. Try typing the
            entire term, or use a different word or phrase.
          </span>
        </div>
      ) : (
        <div className="search-results-container">
          <SearchLegend />
          {hits.map((hit) => (
            <Hit
              key={hit.objectID}
              hit={hit}
              className={hitsTabIndex === hit.__position ? 'active' : ''}
              closeSearchResults={() => setCancelSearch(true)}
            />
          ))}
        </div>
      )}
    </>
  )
}

const AugmentedHits = connectHits(Hits)
