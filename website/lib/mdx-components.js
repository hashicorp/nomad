import defaultMdxComponents from '@hashicorp/nextjs-scripts/lib/providers/docs'
import Placement from '../components/placement-table'

// Since this configuration is shared across server and client, it uses its own
// module that can be imported without pulling in server-only dependencies

export const MDX_COMPONENTS = defaultMdxComponents({
  product: 'nomad',
  additionalComponents: { Placement },
})
