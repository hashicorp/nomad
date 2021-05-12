import Search from '@hashicorp/react-search'
import useIsMobile from '../../use-is-mobile'

export default function SearchBar({ product }) {
  const isMobile = useIsMobile()

  return (
    <Search
      renderHitContent={({ hit, Highlight }) => (
        <>
          <span className="name">
            <Highlight attribute="page_title" hit={hit} tagName="span" />
          </span>
          <span className="description">
            <Highlight attribute="description" hit={hit} tagName="span" />
          </span>
        </>
      )}
      resolveHitLink={(hit) => ({
        href: {
          pathname: `/${transformIdtoUrl(hit.objectID)}`,
        },
      })}
      placeholder={isMobile ? `Search` : `Search ${product} documentation`}
    />
  )
}

function transformIdtoUrl(id) {
  return id.replace(/\/index$/, '')
}
