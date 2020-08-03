import CallToAction from '@hashicorp/react-call-to-action'
import NomadEnterpriseInfo from 'components/enterprise-info/nomad'
import BasicHero from 'components/basic-hero'

export default function UseCaseLayout({ title, description, children }) {
  return (
    <div id="p-use-case">
      <BasicHero
        heading={title}
        content={description}
        links={[
          {
            text: 'Explore HashiCorp Learn',
            url: 'https://learn.hashicorp.com/nomad',
            type: 'outbound',
          },
          {
            text: 'Explore Documentation',
            url: '/docs',
            type: 'inbound',
          },
        ]}
      />
      <div className="g-grid-container">
        <h2 className="g-type-display-2 features-header">Features</h2>
      </div>
      {children}
      <NomadEnterpriseInfo />
      <CallToAction
        variant="compact"
        heading="Ready to get started?"
        content="Nomad Open Source addresses the technical complexity of managing a mixed type of workloads in production at scale by providing a simple and flexible workload orchestrator across distributed infrastructure and clouds."
        brand="nomad"
        links={[
          {
            text: 'Explore HashiCorp Learn',
            type: 'outbound',
            url: 'https://learn.hashicorp.com/nomad',
          },
          {
            text: 'Explore Documentation',
            type: 'inbound',
            url: '/docs',
          },
        ]}
      />
    </div>
  )
}
