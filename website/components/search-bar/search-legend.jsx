import InlineSvg from '@hashicorp/react-inline-svg'
import enterIcon from './img/search-legend-enter.svg?include'
import pageIcon from './img/search-legend-page.svg?include'
import escapeIcon from './img/search-legend-escape.svg?include'

export default function SearchLegend() {
  return (
    <div className="c-search-legend">
      <figure className="legend-item">
        <InlineSvg src={enterIcon} aria-hidden={true} />
        <p className="g-type-tag-label">
          <span className="visually-hidden">Enter</span>
          to select
        </p>
      </figure>
      <figure className="legend-item">
        <InlineSvg src={pageIcon} aria-hidden={true} />
        <p className="g-type-tag-label">
          <span className="visually-hidden">Tab</span>
          to navigate
        </p>
      </figure>
      <figure className="legend-item">
        <InlineSvg src={escapeIcon} aria-hidden={true} />
        <p className="g-type-tag-label">
          <span className="visually-hidden">Escape</span>
          to dismiss
        </p>
      </figure>
    </div>
  )
}
