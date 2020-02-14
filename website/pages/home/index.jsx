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
        description="A simple and flexible workload orchestrator to deploy and manage containerized, non-containerized, and batch applications at any scale across on-prem and clouds."
        links={[
          {
            text: 'Nomad in 45 seconds',
            url: '#TODO'
          },
          {
            text: 'Request Demo',
            url: '#TODO'
          }
        ]}
      />

      <FeaturesList
        title="Why Nomad?"
        items={[
          {
            title: 'Simple and Lightweight',
            content:
              'Single binary (<30MB) and small footprint. Easy to operate at any scale on prem or on any cloud without additional operational overhead',
            icon: {
              url:
                'https://www.datocms-assets.com/2885/1580146559-background.png#TODO',
              alt: 'TODO'
            }
          },
          {
            title: 'Flexible Workload Support',
            content:
              'Orchestrate containers and beyond. First class support for Windows, Java, VM, Docker, and more',
            icon: {
              url:
                'https://www.datocms-assets.com/2885/1580146559-background.png#TODO',
              alt: 'TODO'
            }
          },
          {
            title: 'Modernize Legacy Application without Rewrite',
            content:
              'Bring all orchestration benefits to existing services. Zero downtime deployment, increased utilization, improved resiliency, and more',
            icon: {
              url:
                'https://www.datocms-assets.com/2885/1580146559-background.png#TODO',
              alt: 'TODO'
            }
          },
          {
            title: 'Native integration with Terraform, Consul, and Vault',
            content:
              'Nomad integrates seamlessly with Terraform, Consul and Vault for provisioning, service networking, and secrets management',
            icon: {
              url:
                'https://www.datocms-assets.com/2885/1580146559-background.png#TODO',
              alt: 'TODO'
            }
          },
          {
            title: 'On-prem and on any cloud with Ease',
            content:
              'Run services across private and public clouds transparently to application developers',
            icon: {
              url:
                'https://www.datocms-assets.com/2885/1580146559-background.png#TODO',
              alt: 'TODO'
            }
          },
          {
            title: 'Easy Federation at scale',
            content:
              'Single command for multi-region, multi-cloud federation. Easily deploy and manage applications globally from any server',
            icon: {
              url:
                'https://www.datocms-assets.com/2885/1580146559-background.png#TODO',
              alt: 'TODO'
            }
          }
        ]}
      />

      <CaseStudyCarousel
        title={`Trusted by startups and the\n world’s largest organizations`}
        caseStudies={[
          {
            quote:
              'Lorem ipsum dolor sit amet, consectetur adipiscing elit. A ullamcorper diam eu in enim purus turpis id aliquam.',
            caseStudyURL: '#some-case-study-url',
            person: {
              firstName: 'John',
              lastName: 'Smith',
              photo:
                'https://www.datocms-assets.com/2885/1580329496-user-placeholder.jpg',
              title: 'Director'
            },
            company: {
              name: 'Jet',
              logo: 'https://www.datocms-assets.com/2885/1510949637-jet.svg',
              monochromaticLogo:
                'https://www.datocms-assets.com/2885/1522341143-jet-black.svg'
            }
          },
          {
            quote:
              'Lorem ipsum dolor sit amet, consectetur adipiscing elit. A ullamcorper diam eu in enim purus turpis id aliquam.',
            caseStudyURL: '#some-case-study-url',
            person: {
              firstName: 'Jane',
              lastName: 'Doe',
              photo:
                'https://www.datocms-assets.com/2885/1580329496-user-placeholder.jpg',
              title: 'CIO'
            },
            company: {
              name: 'PagerDuty',
              logo:
                'https://www.datocms-assets.com/2885/1569351715-pagerduty-greenrgb.svg',
              monochromaticLogo:
                'https://www.datocms-assets.com/2885/1569351725-pagerduty-blackrgb.svg'
            }
          },
          {
            quote:
              'Lorem ipsum dolor sit amet, consectetur adipiscing elit. A ullamcorper diam eu in enim purus turpis id aliquam.',
            caseStudyURL: '#some-case-study-url',
            person: {
              firstName: 'Jack',
              lastName: 'Smith',
              photo:
                'https://www.datocms-assets.com/2885/1580329496-user-placeholder.jpg',
              title: 'Manager'
            },
            company: {
              name: 'SAP Ariba',
              logo:
                'https://www.datocms-assets.com/2885/1580419436-logosap-ariba.svg',
              monochromaticLogo:
                'https://www.datocms-assets.com/2885/1580419436-logosap-ariba.svg'
            }
          }
        ]}
      />

      <MiniCTA
        title="Are you using Nomad?"
        description="Share your success story and receive special Nomad swag."
        link={{
          text: 'Share your Nomad story',
          url: '#TODO',
          type: 'inbound'
        }}
      />

      <section>
        <div className="g-container">
          <UseCases
            theme="nomad"
            items={[
              {
                title: 'Simple Container Orchestration',
                description:
                  'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.',
                image: {
                  alt: null,
                  format: 'png',
                  url:
                    'https://www.datocms-assets.com/2885/1575422126-secrets.png?TODO'
                },
                link: {
                  external: false,
                  title: 'Learn more',
                  url: '#TODO'
                }
              },
              {
                title: 'Non Containerized Application Orchestration',
                description:
                  'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.',
                image: {
                  alt: null,
                  format: 'png',
                  url:
                    'https://www.datocms-assets.com/2885/1575422126-secrets.png?TODO'
                },
                link: {
                  external: false,
                  title: 'Learn more',
                  url: '#TODO'
                }
              },
              {
                title: 'Automated Service Networking with Consul',
                description:
                  'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.',
                image: {
                  alt: null,
                  format: 'png',
                  url:
                    'https://www.datocms-assets.com/2885/1575422126-secrets.png?TODO'
                },
                link: {
                  external: false,
                  title: 'Learn more',
                  url: '#TODO'
                }
              }
            ]}
          />
        </div>
      </section>

      <LearnNomad props="TODO (Design WIP)" />

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
