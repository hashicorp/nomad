import InlineSvg from '@hashicorp/react-inline-svg'
import hashicorpLogo from './img/hashicorp-logo.svg?include'
import TemporarySuite from './temporary_suite'

export default function MegaNav({ product }) {
  return (
    <div className="g-mega-nav">
      <div className="g-nav-inner">
        <a href="https://www.hashicorp.com">
          <InlineSvg src={hashicorpLogo} className="hashicorp-logo" />
        </a>
        <TemporarySuite product={product} />
      </div>
    </div>
  )
}
