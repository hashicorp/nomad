import Subnav from '@hashicorp/react-subnav'
import subnavItems from '../../data/subnav'
import { useRouter } from 'next/router'

export default function NomadSubnav() {
  const router = useRouter()
  return (
    <div className="max-width">
      <Subnav
        titleLink={{
          text: 'nomad',
          url: '/'
        }}
        ctaLinks={[
          { text: 'GitHub', url: 'https://www.github.com/hashicorp/nomad' },
          { text: 'Download', url: '/downloads' }
        ]}
        currentPath={router.pathname}
        menuItemsAlign="right"
        menuItems={subnavItems}
      />
    </div>
  )
}
