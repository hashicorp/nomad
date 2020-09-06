import Search from '@hashicorp/react-search'

export default function SearchBar() {
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
      placeholder="Search Nomad documentation"
    />
  )
}
