import React from 'react'
import LinkWrap from '@hashicorp/react-link-wrap'
import CtaLinks from '../CtaLinks/index.js'
import DropdownTrigger from '../DropdownTrigger/index.js'

class MenuItemsOverflow extends React.Component {
  constructor(props) {
    super(props)
    this.state = { isCollapsed: true }
    this.toggleCollapsed = this.toggleCollapsed.bind(this)
    this.parentRef = React.createRef()
    this.handleClick = this.handleClick.bind(this)
  }

  toggleCollapsed() {
    this.setState({ isCollapsed: !this.state.isCollapsed })
  }

  handleClick(event) {
    //  If already collapsed, clicks outside the modal don't matter
    if (this.state.isCollapsed) return true
    //  If we're not collapsed, and the click is outside the component,
    //  we should ensure that the modal closes
    const isClickOutside = !this.parentRef.current.contains(event.target)
    if (isClickOutside) this.setState({ isCollapsed: true })
  }

  componentDidMount() {
    document.addEventListener('click', this.handleClick)
  }

  componentWillUnmount() {
    document.removeEventListener('click', this.handleClick)
  }

  render() {
    const { menuItems, ctaLinks, product, Link, hideGithubStars } = this.props
    const { isCollapsed } = this.state
    return (
      <div className="menu-items-overflow" ref={this.parentRef}>
        <DropdownTrigger
          isCollapsed={isCollapsed}
          onClick={this.toggleCollapsed}
          text="Menu"
          product={product}
        />
        <div
          className={`dropdown style-dropdown ${
            isCollapsed ? 'is-collapsed' : ''
          }`}
        >
          <ul>
            {menuItems.map((menuItem, stableIdx) => {
              if (menuItem === 'divider') return null
              const { text, url, submenu } = menuItem
              if (submenu) {
                //  If we have a submenu, we need to flatten it
                return (
                  // eslint-disable-next-line react/no-array-index-key
                  <div key={stableIdx}>
                    <div className="submenu-title g-type-label">{text}</div>
                    <hr className="divider" />
                    {submenu.map((subItem, subStableIdx) => (
                      <SubmenuItem
                        // eslint-disable-next-line react/no-array-index-key
                        key={subStableIdx}
                        url={subItem.url}
                        text={subItem.text}
                        product={product}
                        Link={Link}
                      />
                    ))}
                    <hr className="divider" />
                  </div>
                )
              } else {
                return (
                  <SubmenuItem
                    // eslint-disable-next-line react/no-array-index-key
                    key={stableIdx}
                    url={url}
                    text={text}
                    product={product}
                    Link={Link}
                  />
                )
              }
            })}
          </ul>
          <CtaLinks
            links={ctaLinks}
            isInDropdown={true}
            product={product}
            hideGithubStars={hideGithubStars}
          />
        </div>
      </div>
    )
  }
}

function SubmenuItem(props) {
  const { url, text, product, Link } = props
  return (
    <li>
      <LinkWrap
        Link={Link}
        className="submenu-item g-type-body-small-strong style-menu-item"
        href={url}
        title={text}
      >
        <span className={`text brand-${product}`}>{text}</span>
      </LinkWrap>
    </li>
  )
}

export default MenuItemsOverflow
