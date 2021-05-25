import LinkWrap from '@hashicorp/react-link-wrap'
import InlineSvg from '@hashicorp/react-inline-svg'

import ConsulLogo from '@hashicorp/mktg-logos/product/consul/primary/color.svg?include'
import HCPLogo from '@hashicorp/mktg-logos/product/hcp/primary/black.svg?include'
import NomadLogo from '@hashicorp/mktg-logos/product/nomad/primary/color.svg?include'
import PackerLogo from '@hashicorp/mktg-logos/product/packer/primary/color.svg?include'
import TerraformLogo from '@hashicorp/mktg-logos/product/terraform/primary/color.svg?include'
import VagrantLogo from '@hashicorp/mktg-logos/product/vagrant/primary/color.svg?include'
import VaultLogo from '@hashicorp/mktg-logos/product/vault/primary/color.svg?include'
import BoundaryLogo from '@hashicorp/mktg-logos/product/boundary/primary/color.svg?include'
import WaypointLogo from '@hashicorp/mktg-logos/product/waypoint/primary/color.svg?include'
import TerraformCloudLogo from '@hashicorp/mktg-logos/product/terraform-cloud/primary/color.svg?include'

const logoDict = {
  boundary: BoundaryLogo,
  consul: ConsulLogo,
  hcp: HCPLogo,
  nomad: NomadLogo,
  packer: PackerLogo,
  terraform: TerraformLogo,
  tfc: TerraformCloudLogo,
  vagrant: VagrantLogo,
  vault: VaultLogo,
  waypoint: WaypointLogo,
}

function TitleLink(props) {
  const { text, url, product, Link } = props
  const Logo = logoDict[text.toLowerCase()]
  return (
    <LinkWrap
      Link={Link}
      className={`title-link brand-${product} ${!Logo ? 'is-text' : ''}`}
      href={url}
      title={text}
    >
      {Logo ? <InlineSvg src={Logo} /> : text}
    </LinkWrap>
  )
}

export default TitleLink
