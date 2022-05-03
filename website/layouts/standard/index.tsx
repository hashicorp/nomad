import query from './query.graphql'
import ProductSubnav from 'components/subnav'
import Footer from 'components/footer'
import { open } from '@hashicorp/react-consent-manager'

export default function StandardLayout(props: Props): React.ReactElement {
  const { useCaseNavItems } = props.data
  return (
    <>
      <ProductSubnav
        menuItems={[
          { text: 'Overview', url: '/', type: 'inbound' },
          {
            text: 'Use Cases',
            submenu: [
              ...useCaseNavItems.map((item) => {
                return {
                  text: item.text,
                  url: `/use-cases/${item.url}`,
                }
              }),
            ].sort((a, b) => a.text.localeCompare(b.text)),
          },
          {
            text: 'Enterprise',
            url: 'https://www.hashicorp.com/products/nomad/',
            type: 'outbound',
          },
          'divider',
          {
            text: 'Tutorials',
            url: 'https://learn.hashicorp.com/nomad',
            type: 'outbound',
          },
          {
            text: 'Docs',
            url: '/docs',
            type: 'inbound',
          },
          {
            text: 'API',
            url: '/api-docs',
            type: 'inbound',
          },
          {
            text: 'Plugins',
            url: '/plugins',
            type: 'inbound',
          },
          {
            text: 'Tools',
            url: '/tools',
            type: 'inbound',
          },
          {
            text: 'Community',
            url: '/community',
            type: 'inbound',
          },
        ]}
      />
      {props.children}
      <Footer openConsentManager={open} />
    </>
  )
}

StandardLayout.rivetParams = {
  query,
  dependencies: [],
}

interface Props {
  children: React.ReactChildren
  data: {
    useCaseNavItems: Array<{ url: string; text: string }>
  }
}
