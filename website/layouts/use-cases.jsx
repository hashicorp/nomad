import CallToAction from '@hashicorp/react-call-to-action'
import NomadEnterpriseInfo from '../components/enterprise-info/nomad'

export default function UseCaseLayout({ title, description, children }) {
  return (
    <div id="p-use-case">
      <CallToAction
        variant="centered"
        heading={title}
        content={description}
        brand="nomad"
        links={[
          {
            text: 'Explore Nomad Learn',
            url: 'https://learn.hashicorp.com/nomad'
          },
          {
            text: 'Explore Documentation',
            url: '/docs'
          }
        ]}
      />
      {children}
      <NomadEnterpriseInfo />
      <CallToAction
        variant="compact"
        heading="Ready to get started?"
        content="Lorem ipsum dolor sit amet, consectetur adipiscing elit. Ipsum mattis quis nibh commodo fermentum. TODO."
        brand="nomad"
        links={[
          {
            text: 'Explore Nomad Learn',
            type: 'outbound',
            url: 'https://learn.hashicorp.com/nomad'
          },
          {
            text: 'Explore Documentation',
            type: 'inbound',
            url: '/docs'
          }
        ]}
      />
    </div>
  )
}
