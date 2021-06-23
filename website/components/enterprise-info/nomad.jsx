import EnterpriseInfo from './index.jsx'

const technicalComplexity = {
  title: 'Technical Complexity',
  label: 'Open Source',
  imageUrl: require('./img/basic.svg?url'),
  description:
    'Nomad Open Source addresses the technical complexity of workload orchestration across the cloud, on-prem, and hybrid infrastructure.',
  link: {
    text: 'View Open Source Features',
    url: 'https://www.hashicorp.com/products/nomad/pricing/',
    type: 'outbound',
  },
}

const organizationalComplexity = {
  title: 'Organizational Complexity',
  label: 'Enterprise',
  imageUrl: require('./img/complex.svg?url'),
  description:
    'Nomad Enterprise addresses the complexity of collaboration and governance across multi-team and multi-cluster deployments.',
  link: {
    text: 'View Enterprise Features',
    url: 'https://www.hashicorp.com/products/nomad/pricing/',
    type: 'outbound',
  },
}

export default function NomadEnterpriseInfo() {
  return (
    <EnterpriseInfo
      title="When to consider Nomad Enterprise?"
      itemOne={technicalComplexity}
      itemTwo={organizationalComplexity}
    />
  )
}
