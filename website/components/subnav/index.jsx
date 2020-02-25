import Subnav from '@hashicorp/react-subnav'
import { useRouter } from 'next/router'

export default function NomadSubnav() {
  const router = useRouter()
  return (
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
      menuItems={[
        { text: 'Overview', url: '/', type: 'inbound' },
        {
          text: 'Use Cases',
          submenu: [
            {
              text: 'Simple Container Orchestration',
              url: '/use-cases/simple-container-orchestration'
            },
            {
              text: 'Non-Containerized Application Orchestration',
              url: '/use-cases/non-containerized-application-orchestration'
            },
            {
              text: 'Automated Service Networking with Consul',
              url: '/use-cases/automated-service-networking-with-consul'
            }
          ]
        },
        {
          text: 'Enterprise',
          url: 'https://www.hashicorp.com/products/nomad/',
          type: 'outbound'
        },
        'divider',
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
          text: 'API',
          url: '/api-docs',
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
