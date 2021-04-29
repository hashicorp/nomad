import UseCasesLayout from 'components/use-case-page'
import TextSplitWithCode from '@hashicorp/react-text-split-with-code'
import TextSplitWithImage from '@hashicorp/react-text-split-with-image'
import FeaturedSlider from '@hashicorp/react-featured-slider'
// Imports below are used in getStaticProps only
import highlightData from '@hashicorp/nextjs-scripts/prism/highlight-data'

export async function getStaticProps() {
  const codeBlocksRaw = {
    containerOrchestration: {
      code:
        'task "webservice" {\n  driver = "docker"\n\n  config {\n    image = "redis:3.2"\n    labels {\n      group = "webservice-cache"\n    }\n  }\n}',
      language: 'hcl',
    },
    windowsSupport: {
      code:
        'sc.exe start "Nomad"\n\nSERVICE_NAME: Nomad\n      TYPE               : 10  WIN32_OWN_PROCESS\n      STATE              : 4  RUNNING\n                              (STOPPABLE, NOT_PAUSABLE, ACCEPTS_SHUTDOWN)\n      WIN32_EXIT_CODE    : 0  (0x0)\n      SERVICE_EXIT_CODE  : 0  (0x0)\n      CHECKPOINT         : 0x0\n      WAIT_HINT          : 0x0\n      PID                : 8008\n      FLAGS              :',
    },
    multiRegionFederation: {
      code: 'nomad server join 1.2.3.4:4648',
    },
  }
  const codeBlocks = await highlightData(codeBlocksRaw)
  return { props: { codeBlocks } }
}

export default function SimpleContainerOrchestrationPage({ codeBlocks }) {
  return (
    <UseCasesLayout
      title="Simple Container Orchestration"
      description="Nomad runs as a single binary with a small resource footprint. Developers use a declarative job specification to define how an application should be deployed.  Nomad handles deployment and automatically recovers applications from failures."
    >
      <TextSplitWithCode
        textSplit={{
          heading: 'Container Orchestration',
          textSide: 'right',
          content:
            'Deploy, manage, and scale your containers with the Docker, Podman, or Singularity task driver.',
          links: [
            {
              text: 'Read More',
              url: 'https://learn.hashicorp.com/collections/nomad/manage-jobs',
              type: 'outbound',
            },
          ],
        }}
        codeBlock={codeBlocks.containerOrchestration}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Run On-Premise with Ease',
          textSide: 'left',
          content:
            'Install and run Nomad easily on bare metal as a single binary and with the same ease as on cloud.',
        }}
        image={{
          url: require('./img/run-on-prem-with-ease.png'),
          alt: '',
        }}
      />

      <TextSplitWithCode
        textSplit={{
          heading: 'Windows Support',
          textSide: 'right',
          content:
            'Deploy Windows containers and processes or run Nomad as a native Windows service with Service Control Manager and NSSM.',
          links: [
            {
              text: 'Watch Jet.com use case',
              url:
                'https://www.hashicorp.com/resources/running-windows-microservices-on-nomad-at-jet-com',
              type: 'outbound',
            },
          ],
        }}
        codeBlock={codeBlocks.windowsSupport}
      />

      <TextSplitWithCode
        textSplit={{
          heading: 'Multi-Region Federation',
          content:
            'Federate Nomad clusters across regions with a single CLI command to deploy applications globally.',
          textSide: 'left',
          links: [
            {
              text: 'Read more',
              url: 'https://learn.hashicorp.com/tutorials/nomad/federation',
              type: 'outbound',
            },
          ],
        }}
        codeBlock={codeBlocks.multiRegionFederation}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Edge Deployment with Simple Topology',
          content:
            'Deploy Nomad with a simple cluster topology on hybrid infrastructure to place workloads to the cloud or at the edge.',
          textSide: 'right',
        }}
        image={{
          url: require('./img/edge.png'),
          alt: '',
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Zero Downtime Deployments',
          content:
            'Achieve zero downtime deployments for applications through rolling updates, blue/green, or canary deployment strategies.',
          textSide: 'left',
          links: [
            {
              text: 'Read more',
              url: 'https://learn.hashicorp.com/collections/nomad/job-updates',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/zero-downtime.png'),
          alt: 'Zero Downtime Deployments',
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'High Performance Batch Workloads',
          content:
            'Run batch jobs with proven scalability of thousands of deployments per second via the batch scheduler.',
          textSide: 'right',
          links: [
            {
              text: 'Watch tech presentation from Citadel',
              url:
                'https://www.hashicorp.com/resources/end-to-end-production-nomad-citadel',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/batch-workloads@3x.png'),
          alt: '',
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Run Specialized Hardware with Device Plugins',
          content:
            'Run GPU and other specialized workloads using Nomad’s device plugins.',
          textSide: 'left',
          links: [
            {
              text: 'Read more',
              url: '/docs/devices',
              type: 'inbound',
            },
          ],
        }}
        image={{
          url: require('./img/specialized-hardware.png'),
          alt: 'Specialized Hardware',
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Run Stateful Workloads',
          content:
            'Natively connect and run stateful services with storage volumes from third-party providers via the Container Storage Interface plugin system.',
          textSide: 'right',
        }}
        image={{
          url: require('./img/stateful-workloads@3x.png'),
          alt: 'Stateful Workloads',
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Flexible Networking Capabilities',
          content:
            'Deploy containerized applications with customized network configurations from third-party vendors via Container Network Interface plugin system',
        }}
        image={{
          url: require('./img/networking-capabilities@3x.png'),
          alt: 'Flexible Networking Capabilities',
        }}
      />

      <FeaturedSlider
        heading="Case Studies"
        theme="dark"
        features={[
          {
            logo: {
              url:
                'https://www.datocms-assets.com/2885/1582097215-roblox-white.svg',
              alt: 'Roblox',
            },
            image: {
              url:
                'https://www.datocms-assets.com/2885/1582096961-roblox-case-study.jpg',
              alt: 'Roblox Nomad Case Study',
            },
            heading: 'Roblox',
            content:
              'Scale a global gaming platform easily and reliably with Nomad to serve 100 million monthly active users',
            link: {
              text: 'Read Case Study',
              url: 'https://www.hashicorp.com/case-studies/roblox',
              type: 'outbound',
            },
          },
          {
            logo: {
              url:
                'https://www.datocms-assets.com/2885/1529339316-logocitadelwhite-knockout.svg',
              alt: 'Citadel',
            },
            image: {
              url:
                'https://www.datocms-assets.com/2885/1509052483-hashiconf2017-end-to-end-production-nomad-at-citadel.jpg',
              alt: 'Citadel Presentation',
            },
            heading: 'Citadel',
            content:
              'Optimize the cost efficiency of batch processing at scale with a hybrid, multi-cloud deployment with Nomad',
            link: {
              text: 'Learn More',
              url:
                'https://www.hashicorp.com/resources/end-to-end-production-nomad-citadel',
              type: 'outbound',
            },
          },
          {
            logo: {
              url:
                'https://www.datocms-assets.com/2885/1594247944-better-help-white.png',
              alt: 'BetterHelp',
            },
            image: {
              url:
                'https://www.datocms-assets.com/2885/1594247996-betterhelp-case-study-screen.png',
              alt: 'BetterHelp Presentation',
            },
            heading: 'BetterHelp',
            content:
              'From 6 dedicated servers in a colocation facility to a cloud-based deployment workflow with Nomad',
            link: {
              text: 'Learn More',
              url:
                'https://www.hashicorp.com/resources/betterhelp-s-hashicorp-nomad-use-case/',
              type: 'outbound',
            },
          },
        ]}
      />
    </UseCasesLayout>
  )
}
