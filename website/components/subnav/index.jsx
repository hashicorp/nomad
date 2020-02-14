import Subnav from '@hashicorp/react-subnav'
import { useRouter } from 'next/router'

export default function NomadSubnav() {
  const router = useRouter()
  return (
    <Subnav
      titleLink={{ text: 'nomad' }}
      ctaLinks={[
        { text: 'GitHub', url: 'https://www.github.com/hashicorp/nomad' },
        { text: 'Download', url: '#TODO' }
      ]}
      currentPath={router.pathname}
      menuItems={[
        { text: 'Overview', url: '/', type: 'inbound' },
        {
          text: 'Use Cases',
          url: '#TODO',
          type: 'inbound'
        },
        {
          text: 'Learn',
          url: 'https://learn.hashicorp.com/nomad',
          type: 'outbound'
        },
        {
          text: 'Docs',
          url: '/docs',
          type: 'inbound'
        },
        {
          text: 'Resources',
          url: '/resources',
          type: 'inbound'
        }
      ]}
    />
  )
}
