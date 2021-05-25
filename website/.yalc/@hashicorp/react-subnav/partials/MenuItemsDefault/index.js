import React from 'react'
import LinkWrap from '@hashicorp/react-link-wrap'
import DropdownTrigger from '../DropdownTrigger/index.js'

function MenuItemsDefault(props) {
  const { menuItems, product, Link, menuItemsAlign } = props
  return (
    <ul className={`menu-items-default menu-items-align-${menuItemsAlign}`}>
      {menuItems.map((menuItem, stableIdx) => {
        if (menuItem === 'divider') {
          // eslint-disable-next-line react/no-array-index-key
          return <VerticalDivider key={stableIdx} />
        }
        const { text, url, submenu, _isActiveUrl } = menuItem
        return menuItem.submenu ? (
          <NavLinkWithDropdown
            // This array is stable, so we can use index as key
            // eslint-disable-next-line react/no-array-index-key
            key={stableIdx}
            text={text}
            submenu={submenu}
            product={product}
            Link={Link}
          />
        ) : (
          <NavLink
            // This array is stable, so we can use index as key
            // eslint-disable-next-line react/no-array-index-key
            key={stableIdx}
            text={text}
            url={url}
            isActive={_isActiveUrl}
            product={product}
            Link={Link}
          />
        )
      })}
    </ul>
  )
}

function VerticalDivider() {
  return <span className="vertical-divider" />
}

function NavLink(props) {
  const { text, url, product, isActive, Link } = props
  return (
    <li>
      <LinkWrap
        Link={Link}
        className={`nav-link g-type-body-small-strong style-menu-item ${
          isActive ? 'is-active' : ''
        }`}
        href={url}
      >
        <span className={`text brand-${product} `}>{text}</span>
      </LinkWrap>
    </li>
  )
}

class NavLinkWithDropdown extends React.Component {
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
    const { text, submenu, product, Link } = this.props
    const { isCollapsed } = this.state
    const hasActiveChild = submenu.reduce((acc, s) => {
      return s._isActiveUrl || acc
    }, false)
    return (
      <li ref={this.parentRef}>
        <DropdownTrigger
          onClick={this.toggleCollapsed}
          isCollapsed={isCollapsed}
          text={text}
          product={product}
          isActive={hasActiveChild}
        />
        <ul
          className={`submenu-modal style-dropdown ${
            isCollapsed ? 'is-collapsed' : ''
          }`}
        >
          {submenu.map((submenuItem, stableIdx) => {
            const { text, url } = submenuItem
            return (
              // eslint-disable-next-line react/no-array-index-key
              <li key={stableIdx}>
                <LinkWrap
                  Link={Link}
                  className="g-type-body-small-strong style-menu-item"
                  href={url}
                >
                  <span className={`text brand-${product}`}>{text}</span>
                </LinkWrap>
              </li>
            )
          })}
        </ul>
      </li>
    )
  }
}

export default MenuItemsDefault
