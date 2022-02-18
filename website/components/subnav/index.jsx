import Subnav from '@hashicorp/react-subnav'
import subnavItems from '../../data/subnav'
import { useRouter } from 'next/router'

export default function NomadSubnav() {
  const router = useRouter()
  return (
    <Subnav
      titleLink={{
        text: 'HashiCorp Nomad',
        url: '/',
      }}
      ctaLinks={[
        { text: 'GitHub', url: 'https://www.github.com/hashicorp/nomad' },
        {
          text: 'Download',
          url: '/downloads',
          theme: {
            brand: 'nomad',
          },
        },
      ]}
      currentPath={router.asPath}
      menuItemsAlign="right"
      menuItems={subnavItems}
      constrainWidth
      matchOnBasePath
    />
  )
}
