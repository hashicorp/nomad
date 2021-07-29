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

// Docker not needed? Exec?

export default function SimpleContainerOrchestrationPage({ codeBlocks }) {
  return (
    <UseCasesLayout
      title="Edge Compute"
      description="Manage workloads across the globe, on any device, using a single Nomad cluster."
    >
      <TextSplitWithImage
        textSplit={{
          heading: 'Global Deployment with Simple Topology',
          content:
            'A single Nomad cluster can manage clients across regions with ease. This architecture makes tracking and reasoning about your global deployments simple. It has been proven to scale to two million containers running globally over 6000 nodes in the Two Million Container Challenge.',
          textSide: 'left',
        }}
        image={{
          url: require('./img/edge.png'),
          alt: '',
        }}
      />

      <TextSplitWithCode
        textSplit={{
          heading: 'Run Anywhere, from Raspberry Pis to Supercomputers',
          textSide: 'right',
          content:
            'Nomad runs on a wide variety of architectures: Linux, ARM, Windows, Darwin. It\'s small resource footprint allows it to efficiently run on a IOT Devices with plenty of cycles to spare.',
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
          heading: 'High Latency? No Problem.',
          textSide: 'left',
          content:
            'Nomad is built to handle high latency environments. Even if your clients disconnect for minutes (or hours!) regularly, Nomad can gracefully handle reschedules and reconnections.',
        }}
        image={{
          url: require('./img/run-on-prem-with-ease.png'),
          alt: '',
        }}
      />

      <TextSplitWithCode
        textSplit={{
          heading: 'Get the most out of your devices',
          textSide: 'right',
          content:
            '',
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
          heading: 'Docker not needed? Exec?',
          content: 'Low overhead',
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
