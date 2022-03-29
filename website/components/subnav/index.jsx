import Subnav from '@hashicorp/react-subnav'
import { useRouter } from 'next/router'

export default function NomadSubnav({ menuItems }) {
  const router = useRouter()
  return (
    <Subnav
      hideGithubStars={true}
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
      menuItems={menuItems}
      constrainWidth
      matchOnBasePath
    />
  )
}
