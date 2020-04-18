import FeaturesList from '../../components/features-list'
import HomepageHero from '../../components/homepage-hero'
import CaseStudyCarousel from '../../components/case-study-carousel'
import UseCases from '@hashicorp/react-use-cases'
import MiniCTA from '../../components/mini-cta'
import NomadEnterpriseInfo from '../../components/enterprise-info/nomad'
import LearnNomad from '../../components/learn-nomad'
import CallToAction from '@hashicorp/react-call-to-action'

export default function Homepage() {
  return (
    <div id="p-home">
      <HomepageHero
        title="Workload Orchestration Made Easy"
        description="A simple and flexible workload orchestrator to deploy and manage containers and non-containerized applications across on-prem and clouds at scale."
        links={[
          {
            text: 'Download',
            url: '/downloads',
            type: 'download'
          },
          {
            text: 'Get Started',
            url: 'https://learn.hashicorp.com/nomad',
            type: 'outbound'
          }
        ]}
      />

      <FeaturesList
        title="Why Nomad?"
        items={[
          {
            title: 'Simple and Lightweight',
            content:
              'Single 35MB binary that integrates into existing infrastructure.  Easy to operate on-prem or in the cloud with minimal overhead.',
            icon: require('./img/why-nomad/simple-and-lightweight.svg')
          },
          {
            title: 'Flexible Workload Support',
            content:
              'Orchestrate applications of any type - not just containers. First class support for Docker, Windows, Java, VMs, and more.',
            icon: require('./img/why-nomad/flexible-workload-support.svg')
          },
          {
            title: 'Modernize Legacy Applications without Rewrite',
            content:
              'Bring orchestration benefits to existing services. Achieve zero downtime deployments, improved resilience, higher resource utilization, and more without containerization.',
            icon: require('./img/why-nomad/modernize-legacy-applications.svg')
          },
          {
            title: 'Easy Federation at Scale',
            content:
              'Single command for multi-region, multi-cloud federation. Deploy applications globally to any region using Nomad as a single unified control plane.',
            icon: require('./img/why-nomad/federation.svg')
          },
          {
            title: 'Multi-Cloud with Ease',
            content:
              'One single unified workflow for deploying to bare metal or cloud environments. Enable multi-cloud applications with ease.',
            icon: require('./img/why-nomad/servers.svg')
          },
          {
            title: 'Native Integrations with Terraform, Consul, and Vault',
            content:
              'Nomad integrates seamlessly with Terraform, Consul and Vault for provisioning, service networking, and secrets management.',
            icon: require('./img/why-nomad/native-integration.svg')
          }
        ]}
      />

      <CaseStudyCarousel
        title="Trusted by startups and the world’s largest organizations"
        caseStudies={[
          {
            quote:
              'Kubernetes is the 800-pound gorilla of container orchestration, coming with a price tag. So we looked into alternatives - and fell in love with Nomad.',
            caseStudyURL:
              'https://endler.dev/2019/maybe-you-dont-need-kubernetes/',
            person: {
              firstName: 'Matthias',
              lastName: 'Endler',
              photo:
                'https://www.datocms-assets.com/2885/1582163422-matthias-endler.png',
              title: 'Backend Engineer'
            },
            company: {
              name: 'Trivago',
              logo: 'https://www.datocms-assets.com/2885/1582162145-trivago.svg'
            }
          },
          {
            quote:
              'We have people who are first-time system administrators deploying applications. There is a guy on our team who worked in IT help desk for 8 years - just today he upgraded an entire cluster himself.',
            caseStudyURL: 'https://www.hashicorp.com/case-studies/roblox/',
            person: {
              firstName: 'Rob',
              lastName: 'Cameron',
              photo:
                'https://www.datocms-assets.com/2885/1582180216-rob-cameron.jpeg',
              title: 'Technical Director of Infrastructure'
            },
            company: {
              name: 'Roblox',
              logo:
                'https://www.datocms-assets.com/2885/1582180369-roblox-color.svg'
            }
          },
          {
            quote:
              'Our customers’ jobs are changing constantly. It’s challenging to dynamically predict demand, what types of jobs, and the resource requirements. We found that Nomad excelled in this area.',
            caseStudyURL:
              'https://www.hashicorp.com/resources/nomad-vault-circleci-security-scheduling',
            person: {
              firstName: 'Rob',
              lastName: 'Zuber',
              photo:
                'https://www.datocms-assets.com/2885/1582180618-rob-zuber.jpeg',
              title: 'CTO'
            },
            company: {
              name: 'CircleCI',
              logo:
                'https://www.datocms-assets.com/2885/1582180745-circleci-logo.svg'
            }
          },
          {
            quote:
              'Adopting Nomad did not require us to change our packaging format — we could continue to package Python in Docker and build binaries for the rest of our applications.',
            caseStudyURL:
              'https://medium.com/@copyconstruct/schedulers-kubernetes-and-nomad-b0f2e14a896',
            person: {
              firstName: 'Cindy',
              lastName: 'Sridharan',
              photo:
                'https://www.datocms-assets.com/2885/1582181517-cindy-sridharan.png',
              title: 'Engineer'
            },
            company: {
              name: 'imgix',
              logo: 'https://www.datocms-assets.com/2885/1582181250-imgix.svg'
            }
          }
        ]}
        featuredLogos={[
          {
            companyName: 'Trivago',
            url:
              'https://www.datocms-assets.com/2885/1582162317-trivago-monochromatic.svg'
          },
          {
            companyName: 'Roblox',
            url:
              'https://www.datocms-assets.com/2885/1582180373-roblox-monochrome.svg'
          },
          {
            companyName: 'CircleCI',
            url:
              'https://www.datocms-assets.com/2885/1582180745-circleci-logo.svg'
          },
          {
            companyName: 'SAP Ariba',
            url:
              'https://www.datocms-assets.com/2885/1580419436-logosap-ariba.svg'
          },
          {
            companyName: 'Pandora',
            url:
              'https://www.datocms-assets.com/2885/1523044075-pandora-black.svg'
          },
          {
            companyName: 'Citadel',
            url:
              'https://www.datocms-assets.com/2885/1582323352-logocitadelwhite-knockout.svg'
          },
          {
            companyName: 'Jet',
            url: 'https://www.datocms-assets.com/2885/1522341143-jet-black.svg'
          },
          {
            companyName: 'Deluxe',
            url:
              'https://www.datocms-assets.com/2885/1582323254-deluxe-logo.svg'
          }
        ]}
      />

      <MiniCTA
        title="Are you using Nomad in production?"
        link={{
          text: 'Share your success story and receive special Nomad swag.',
          url: 'https://forms.gle/rdaLSuMGpvbomgYk9',
          type: 'outbound'
        }}
      />

      <div className="use-cases g-grid-container">
        <h2 className="g-type-display-2">Use Cases</h2>
        <UseCases
          theme="nomad"
          items={[
            {
              title: 'Simple Container Orchestration',
              description:
                'Deploy, manage, and scale enterprise containers in production with ease.',
              image: {
                alt: null,
                format: 'png',
                url: require('./img/use-cases/simple-container-orchestration.svg')
              },
              link: {
                external: false,
                title: 'Learn more',
                url: '/use-cases/simple-container-orchestration'
              }
            },
            {
              title: 'Non Containerized Application Orchestration',
              description:
                'Modernize non-containerized applications without rewrite.',
              image: {
                alt: null,
                format: 'png',
                url: require('./img/use-cases/non-containerized-application-orchestration.svg')
              },
              link: {
                external: false,
                title: 'Learn more',
                url: '/use-cases/non-containerized-application-orchestration'
              }
            },
            {
              title: 'Automated Service Networking with Consul',
              description:
                'Service discovery and service mesh with HashiCorp Consul to ensure secure service-to-service communication.',
              image: {
                alt: null,
                format: 'png',
                url: require('./img/use-cases/automated-service-networking-with-consul.svg')
              },
              link: {
                external: false,
                title: 'Learn more',
                url: '/use-cases/automated-service-networking-with-consul'
              }
            }
          ]}
        />
      </div>

      <LearnNomad
        items={[
          {
            title: 'Getting Started',
            category: 'Step-by-Step Guides',
            time: '24 mins',
            link:
              'https://learn.hashicorp.com/nomad?track=getting-started#getting-started',
            image: require('./img/learn-nomad/cap.svg')
          },
          {
            title: 'Deploy and Manage Nomad Jobs',
            category: 'Step-by-Step Guides',
            time: '36 mins',
            link:
              'https://learn.hashicorp.com/nomad?track=managing-jobs#getting-started',
            image: require('./img/learn-nomad/cubes.svg')
          }
        ]}
      />

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
