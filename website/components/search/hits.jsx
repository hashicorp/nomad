import Link from 'next/link'
import { forwardRef, useEffect, useRef, useState } from 'react'
import { Highlight, connectHits } from 'react-instantsearch-dom'
import generateSlug from '@hashicorp/remark-plugins/generate_slug'
import InlineSvg from '@hashicorp/react-inline-svg'
import ReturnIcon from './img/return.svg?include'
import SearchLegend from './legend'
import { useSearch } from './provider'

function Hits({
  /* Props provided from connector */
  hits,
  /* Props passed explicity */
  handleEscape,
  searchQuery,
  setCancelSearch,
}) {
  const selectedHit = useRef(null)
  const [hitsTabIndex, setHitsTabIndex] = useState(null)

  useEffect(() => {
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [hitsTabIndex])

  useEffect(() => {
    if (selectedHit?.current) scrollToActive(selectedHit.current)
  }, [hitsTabIndex])

  function onKeyDown(e) {
    switch ([e.ctrlKey, e.keyCode].join(',')) {
      // [Enter]
      case 'false,13':
        return handleEnter(e)
      // [Escape]
      case 'false,27':
        setHitsTabIndex(null)
        return handleEscape()
      // [ArrowDown]
      // [Ctrl-n]
      case 'false,40':
      case 'true,78':
        if (!hitsTabIndex) {
          setHitsTabIndex(0)
          scrollToActive()
        }
        return incrementTabIndex()
      // [ArrowUp]
      // [Ctrl-p]
      case 'false,38':
      case 'true,80':
        e.preventDefault()
        return decrementTabIndex()
    }
  }

  function handleEnter(e) {
    e.preventDefault()
    selectedHit.current?.click()
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
      block: 'nearest',
      inline: 'start',
    })
  }

  return (
    <div className="c-hits">
      {hits.length === 0 ? (
        <div className="no-hits">
          <span className="title">{`No results for ${searchQuery}...`}</span>
          <span className="message">
            Search tips: some terms require an exact match. Try typing the
            entire term, or use a different word or phrase.
          </span>
        </div>
      ) : (
        <div className="hits">
          <SearchLegend />
          <ul className="hits-list">
            {hits.map((hit) => (
              <Hit
                key={hit.objectID}
                hit={hit}
                closeSearchResults={() => setCancelSearch(true)}
                {...(hitsTabIndex === hit.__position && {
                  className: 'active',
                  ref: selectedHit,
                })}
              />
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}

export default connectHits(Hits)

//  we need an `a` tag that also has a click handler
//  next/link passes a click handler to its child; so in order to merge ours in, we need this syntax
//  ref: https://github.com/zeit/next.js/#with-link
const LinkWithClick = forwardRef(({ children, ...props }, ref) => (
  <a {...props} ref={ref}>
    {children}
  </a>
))

const Hit = forwardRef(({ hit, className = '', closeSearchResults }, ref) => {
  const { logClick, setSearchQuery } = useSearch()
  let hitUrl = `/${hit.objectID}`

  // We append an associated heading slug to the hitUrl if and only if the search result matches one heading
  // and does not match either description or page title criteria
  if (
    hit?._highlightResult?.description?.matchLevel === 'none' &&
    hit?._highlightResult?.page_title?.matchLevel === 'none'
  ) {
    const matchedHeading = hit.headings.filter((heading, idx) => {
      return hit?._highlightResult?.headings[idx]?.matchLevel !== 'none'
    })

    if (matchedHeading.length === 1) {
      hitUrl = `${hitUrl}#${generateSlug(matchedHeading[0])}`
    }
  }

  const handleClick = () => {
    logClick(hit)
    closeSearchResults()
    setSearchQuery('')
  }

  return (
    <li className="hit-item">
      <Link href={`${hitUrl}?searchQueryId=${hit.__queryID}`} as={hitUrl}>
        <LinkWithClick
          ref={ref}
          className={`hit-link-wrapper ${className}`}
          href={hitUrl}
          onClick={handleClick}
        >
          <div className="hit">
            <div className="hit-content">
              <span className="name">
                <Highlight attribute="page_title" hit={hit} tagName="span" />
              </span>
              <span className="description">
                <Highlight attribute="description" hit={hit} tagName="span" />
              </span>
            </div>
            <InlineSvg className={`icon-return`} src={ReturnIcon} />
          </div>
        </LinkWithClick>
      </Link>
    </li>
  )
})
