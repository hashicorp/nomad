import UseCasesLayout from 'layouts/use-cases'
import TextSplitWithCode from '@hashicorp/react-text-split-with-code'
import TextSplitWithImage from '@hashicorp/react-text-split-with-image'
import FeaturedSliderSection from 'components/featured-slider-section'

export default function SimpleContainerOrchestrationPage() {
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
              url:
                'https://learn.hashicorp.com/nomad?track=managing-jobs#managing-jobs',
              type: 'outbound',
            },
          ],
        }}
        codeBlock={{
          code: `task "webservice" {
  driver = "docker"

  config {
    image = "redis:3.2"
    labels {
      group = "webservice-cache"
    }
  }
}`,
          language: 'hcl',
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Run On-Premise with Ease',
          textSide: 'left',
          content:
            'Install and run Nomad easily on bare metal as a single binary and with the same ease as on cloud.',
        }}
        image={{
          url: require('./img/on-prem-with-ease.svg'),
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
        codeBlock={{
          code: `sc.exe start "Nomad"

SERVICE_NAME: Nomad
      TYPE               : 10  WIN32_OWN_PROCESS
      STATE              : 4  RUNNING
                              (STOPPABLE, NOT_PAUSABLE, ACCEPTS_SHUTDOWN)
      WIN32_EXIT_CODE    : 0  (0x0)
      SERVICE_EXIT_CODE  : 0  (0x0)
      CHECKPOINT         : 0x0
      WAIT_HINT          : 0x0
      PID                : 8008
      FLAGS              :`,
        }}
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
              url:
                'https://learn.hashicorp.com/nomad/operating-nomad/federation',
              type: 'outbound',
            },
          ],
        }}
        codeBlock={{
          code: 'nomad server join 1.2.3.4:4648',
          prefix: 'dollar',
        }}
      />

      <div className="with-border">
        <TextSplitWithImage
          textSplit={{
            heading: 'Edge Deployment with Simple Topology',
            content:
              'Deploy Nomad with a simple cluster topology on hybrid infrastructure to place workloads to the cloud or at the edge.',
            textSide: 'right',
          }}
          image={{
            url: require('./img/edge-deployment.svg'),
            alt: '',
          }}
        />
      </div>

      <TextSplitWithImage
        textSplit={{
          heading: 'Zero Downtime Deployments',
          content:
            'Achieve zero downtime deployments for applications through rolling updates, blue/green, or canary deployment strategies.',
          textSide: 'left',
          links: [
            {
              text: 'Read more',
              url: 'https://learn.hashicorp.com/nomad/update-strategies',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/zero-downtime-deployments.png'),
          alt: 'Zero Downtime Deployments',
        }}
      />

      <div className="with-border">
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
            url: require('./img/high-performance-batch-workloads.png'),
            alt: '',
          }}
        />
      </div>

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
          url: require('./img/csi.svg'),
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
          url: require('./img/cni.svg'),
          alt: 'Flexible Networking Capabilities',
        }}
      />

      <FeaturedSliderSection
        heading="Case Studies"
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
