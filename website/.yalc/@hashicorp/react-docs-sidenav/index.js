import React, { useCallback, useEffect, useRef, useState } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/router'
// @hashicorp imports
import useProductMeta from '@hashicorp/nextjs-scripts/lib/providers/product-meta'
import LinkWrap, { isAbsoluteURL } from '@hashicorp/react-link-wrap'
import InlineSvg from '@hashicorp/react-inline-svg'
// local utilities
import flagActiveNodes from './utils/flag-active-nodes'
import filterContent from './utils/filter-content'
import useEventListener from './utils/use-event-listener'
// svg
import svgMenuIcon from './icons/menu.svg?include'
import svgChevron from './icons/chevron.svg?include'
import svgBullet from './icons/bullet.svg?include'
import svgExternalLink from './icons/external-link.svg?include'
import svgSearchIcon from './icons/search.svg?include'
// styles
import s from './style.module.css'

export default function DocsSidenav({
  currentPath,
  baseRoute,
  product,
  navData,
  disableFilter = false,
  versionSelect,
  search,
}) {
  const router = useRouter()
  // splitting on ? to drop query parameters if they exist
  const activePath = router ? router.asPath.split('?')[0] : null

  // Get theme class
  // ( note: we could consider getting the product prop here,
  // rather than requiring it to be passed in )
  const { themeClass } = useProductMeta(product)

  // Set up filtering state
  const [filterInput, setFilterInput] = useState('')
  const [content, setContent] = useState(navData)
  const [filteredContent, setFilteredContent] = useState(navData)

  // isMobileOpen controls menu open / close state
  const [isMobileOpen, setIsMobileOpen] = useState(false)
  // isMobileFullyHidden reflects if the menu is fully transitioned to a hidden state
  const [isMenuFullyHidden, setIsMenuFullyHidden] = useState(true)

  const [isSearchOpen, setIsSearchOpen] = useState(false)

  // We want to avoid exposing links to keyboard navigation
  // when the menu is hidden on mobile. But we don't want our
  // menu to flash when hide and shown. To meet both needs,
  // we listen for transition end on the menu element, and when
  // a transition ends and the menu is not open, we set isMenuFullyHidden
  // which translates into a visibility: hidden CSS property
  const menuRef = useRef(null)
  const handleMenuTransitionEnd = useCallback(() => {
    setIsMenuFullyHidden(!isMobileOpen)
  }, [isMobileOpen, setIsMenuFullyHidden])
  useEventListener('transitionend', handleMenuTransitionEnd, menuRef.current)

  // Close the menu when there is a click outside
  const handleDocumentClick = useCallback(
    (event) => {
      if (!isMobileOpen) return
      const isClickOutside = !menuRef.current.contains(event.target)
      if (isClickOutside) setIsMobileOpen(false)
    },
    [isMobileOpen]
  )
  useEventListener(
    'click',
    handleDocumentClick,
    typeof window !== 'undefined' ? document : null
  )

  // When client-side navigation occurs,
  // we want to close the mobile rather than keep it open
  useEffect(() => {
    setIsMobileOpen(false)
  }, [activePath])

  // When path-related data changes, update content to ensure
  // `__isActive` props on each content item are up-to-date
  // Note: we could also reset filter input here, if we don't
  // want to filter input to persist across client-side nav, ie:
  // setFilterInput("")
  useEffect(() => {
    if (!navData) return
    setContent(flagActiveNodes(navData, currentPath, activePath))
  }, [currentPath, navData, activePath])

  // When filter input changes, update content
  // to filter out items that don't match
  useEffect(() => {
    setFilteredContent(filterContent(content, filterInput))
  }, [filterInput, content])

  return (
    <div className={`g-docs-sidenav ${s.root} ${themeClass || ''}`}>
      {!isSearchOpen ? (
        <>
          <button
            className={`${s.mobileMenuToggle} g-type-body-small-strong`}
            onClick={() => setIsMobileOpen(!isMobileOpen)}
          >
            <span>
              <InlineSvg src={svgMenuIcon} /> Documentation Menu
            </span>
          </button>
          {search ? (
            <button
              type="button"
              aria-label="Show Search Bar"
              className={s.searchToggle}
              onClick={() => setIsSearchOpen(true)}
            >
              <InlineSvg src={svgSearchIcon} />
            </button>
          ) : null}
        </>
      ) : (
        <>
          {search}
          <button
            type="button"
            className={`${s.searchClose} g-type-body-small-strong`}
            onClick={() => setIsSearchOpen(false)}
          >
            Cancel
          </button>
        </>
      )}
      <ul
        className={s.rootList}
        ref={menuRef}
        data-is-mobile-hidden={!isMobileOpen && isMenuFullyHidden}
        data-is-mobile-open={isMobileOpen}
      >
        <button
          className={s.mobileClose}
          onClick={() => setIsMobileOpen(!isMobileOpen)}
        >
          &times;
        </button>
        {versionSelect}
        {!disableFilter && (
          <input
            className={s.filterInput}
            placeholder="Filter..."
            onChange={(e) => setFilterInput(e.target.value)}
            value={filterInput}
          />
        )}
        <NavTree
          baseRoute={baseRoute}
          content={filteredContent || []}
          currentPath={currentPath}
          Link={Link}
        />
      </ul>
    </div>
  )
}

function NavTree({ baseRoute, content }) {
  return content.map((item, idx) => {
    //  Dividers
    if (item.divider) {
      // eslint-disable-next-line react/no-array-index-key
      return <Divider key={idx} />
    }
    // Direct links
    if (item.title && item.href) {
      return (
        <DirectLink
          key={item.title + item.href}
          title={item.title}
          href={item.href}
          isActive={item.__isActive}
        />
      )
    }
    // Individual pages (leaf nodes)
    if (item.path) {
      return (
        <NavLeaf
          key={item.path}
          title={item.title}
          isActive={item.__isActive}
          isHidden={item.hidden}
          url={`/${baseRoute}/${item.path}`}
        />
      )
    }
    // Otherwise, render a nav branch
    // (this will recurse and render a nav tree)
    return (
      <NavBranch
        key={item.title}
        title={item.title}
        routes={item.routes}
        isActive={item.__isActive}
        isFiltered={item.__isFiltered}
        isHidden={item.hidden}
        baseRoute={baseRoute}
      />
    )
  })
}

function NavBranch({
  title,
  routes,
  baseRoute,
  isActive,
  isFiltered,
  isHidden,
}) {
  const [isOpen, setIsOpen] = useState(false)

  // Ensure categories appear open if they're active
  // or match the current filter
  useEffect(() => setIsOpen(isActive || isFiltered), [isActive, isFiltered])

  return (
    <li className={isHidden ? s.hiddenNode : ''}>
      <button
        className={s.navItem}
        onClick={() => setIsOpen(!isOpen)}
        data-is-open={isOpen}
        data-is-active={isActive}
      >
        <InlineSvg
          src={svgChevron}
          className={s.navBranchIcon}
          data-is-open={isOpen}
          data-is-active={isActive}
        />
        <span dangerouslySetInnerHTML={{ __html: title }} />
      </button>

      <ul className={s.navBranchSubnav} data-is-open={isOpen}>
        <NavTree baseRoute={baseRoute} content={routes} />
      </ul>
    </li>
  )
}

function NavLeaf({ title, url, isActive, isHidden }) {
  // if the item has a path, it's a leaf node so we render a link to the page
  return (
    <li className={isHidden ? s.hiddenNode : ''}>
      <Link href={url}>
        <a className={s.navItem} data-is-active={isActive}>
          <InlineSvg
            src={svgBullet}
            className={s.navLeafIcon}
            data-is-active={isActive}
          />
          <span dangerouslySetInnerHTML={{ __html: title }} />
        </a>
      </Link>
    </li>
  )
}

function DirectLink({ title, href, isActive }) {
  return (
    <li>
      <LinkWrap
        className={s.navItem}
        href={href}
        Link={Link}
        data-is-active={isActive}
      >
        <InlineSvg
          src={svgBullet}
          className={s.navLeafIcon}
          data-is-active={isActive}
        />
        <span dangerouslySetInnerHTML={{ __html: title }} />
        {isAbsoluteURL(href) ? (
          <InlineSvg src={svgExternalLink} className={s.externalLinkIcon} />
        ) : null}
      </LinkWrap>
    </li>
  )
}

function Divider() {
  return <hr className={s.divider} />
}
