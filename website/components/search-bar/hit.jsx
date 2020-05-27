import { forwardRef } from 'react'
import Link from 'next/link'
import { Highlight } from 'react-instantsearch-dom'
import InlineSvg from '@hashicorp/react-inline-svg'
import ReturnIcon from './img/return.svg?include'
import { logClick } from '../../lib/algolia'

//  we need an `a` tag that also has a click handler
//  next/link passes a click handler to its child; so in order to merge ours in, we need this syntax
//  ref: https://github.com/zeit/next.js/#with-link
const LinkWithClick = forwardRef(({ children, ...props }, ref) => (
  <a {...props} ref={ref}>
    {children}
  </a>
))

export default function Hit({ hit, className = '', closeSearchResults }) {
  const hitUrl = `/${hit.__resourcePath.replace('.mdx', '')}`

  const handleClick = () => {
    logClick(hit)
    closeSearchResults()
  }

  return (
    <Link href={`${hitUrl}?searchQueryId=${hit.__queryID}`} as={hitUrl}>
      <LinkWithClick
        className={`hit-link-wrapper ${className}`}
        href={hitUrl}
        onClick={handleClick}
      >
        <div className="hit">
          <div className="hit-content">
            <span className="hit-name">
              <Highlight attribute="name" hit={hit} tagName="span" />
            </span>
            <ul className="hit-badge-group">
              {hit.products_used &&
                hit.products_used.map((product) => (
                  <li
                    className={`hit-badge ${product.toLowerCase()}`}
                    key={product}
                  >
                    {product}
                  </li>
                ))}
            </ul>
            <span className="hit-description">
              <Highlight attribute="description" hit={hit} tagName="span" />
            </span>
          </div>
          <InlineSvg className={`return-icon`} src={ReturnIcon} />
        </div>
      </LinkWithClick>
    </Link>
  )
}
