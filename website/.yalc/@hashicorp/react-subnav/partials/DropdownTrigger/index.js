import classNames from 'classnames'
import InlineSvg from '@hashicorp/react-inline-svg'
import SvgChevronDown from './icons/chevron-down.svg?include'

function DropdownTrigger(props) {
  const { onClick, isCollapsed, text, product, isActive } = props
  return (
    <button
      className={classNames(
        'dropdown-trigger',
        'g-type-body-small-strong',
        `brand-${product}`,
        'style-menu-item',
        { 'is-collapsed': isCollapsed },
        {
          'is-active': isActive,
        }
      )}
      onMouseDown={(e) => e.preventDefault()}
      onClick={onClick}
    >
      <span className={`text brand-${product}`}>{text}</span>
      <InlineSvg src={SvgChevronDown} />
    </button>
  )
}

export default DropdownTrigger
