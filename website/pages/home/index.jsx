import FeaturesList from 'components/features-list'
import HomepageHero from 'components/homepage-hero'
import CaseStudyCarousel from 'components/case-study-carousel'
import UseCases from '@hashicorp/react-use-cases'
import MiniCTA from 'components/mini-cta'
import NomadEnterpriseInfo from 'components/enterprise-info/nomad'
import LearnNomad from 'components/learn-nomad'
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
            type: 'download',
          },
          {
            text: 'Get Started',
            url: 'https://learn.hashicorp.com/nomad',
            type: 'outbound',
          },
        ]}
      />

      <FeaturesList
        title="Why Nomad?"
        intro="<p>Nomad is widely adopted and used in production by organizations in traditional on-premises datacenters, at the edge, and on the cloud.</p><p>Learn why users choose Nomad as an <a href='/docs/nomad-vs-kubernetes'>alternative to Kubernetes.</a></p>"
        items={[
          {
            title: 'Simple and Lightweight',
            content:
              'Single binary that integrates into existing infrastructure. Easy to operate on-prem or in the cloud with minimal overhead.',
            icon: require('./img/why-nomad/simple-and-lightweight.svg'),
          },
          {
            title: 'Flexible Workload Support',
            content:
              'Orchestrate applications of any type - not just containers. First class support for Docker, Windows, Java, VMs, and more.',
            icon: require('./img/why-nomad/flexible-workload-support.svg'),
          },
          {
            title: 'Modernize Legacy Applications without Rewrite',
            content:
              'Bring orchestration benefits to existing services. Achieve zero downtime deployments, improved resilience, higher resource utilization, and more without containerization.',
            icon: require('./img/why-nomad/modernize-legacy-applications.svg'),
          },
          {
            title: 'Easy Federation at Scale',
            content:
              'Single command for multi-region, multi-cloud federation. Deploy applications globally to any region using Nomad as a single unified control plane.',
            icon: require('./img/why-nomad/federation.svg'),
          },
          {
            title: 'Deploy and Scale with Ease',
            content:
              'Deploy to bare metal with the same ease as in cloud environments. Scale globally without complexity. Read <a href="https://www.hashicorp.com/c2m">the 2 Million Container Challenge</a>.',
            icon: require('./img/why-nomad/servers.svg'),
          },
          {
            title: 'Native Integrations with Terraform, Consul, and Vault',
            content:
              'Nomad integrates seamlessly with Terraform, Consul and Vault for provisioning, service networking, and secrets management.',
            icon: require('./img/why-nomad/native-integration.svg'),
          },
        ]}
      />

      <CaseStudyCarousel
        title="Trusted by startups and the world’s largest organizations"
        caseStudies={[
          {
            quote:
              'We deployed a dynamic task scheduling system with Nomad. It helped us improve the availability of distributed services across more than 200 edge cities worldwide.',
            caseStudyURL:
              'https://blog.cloudflare.com/how-we-use-hashicorp-nomad/',
            person: {
              firstName: 'Thomas',
              lastName: 'Lefebvre',
              photo:
                'https://www.datocms-assets.com/2885/1591836195-tlefebvrephoto.jpg',
              title: 'Tech Lead, SRE',
            },
            company: {
              name: 'Cloudflare',
              logo:
                'https://www.datocms-assets.com/2885/1522194205-cf-logo-h-rgb.png',
            },
          },
          {
            quote:
              'We’ve really streamlined our data operations with Nomad and freed up our time to work on more high-impact tasks. Once we launch new microservices, they just work.',
            caseStudyURL:
              'https://www.hashicorp.com/blog/nomad-community-story-navi-capital',
            person: {
              firstName: 'Carlos',
              lastName: 'Domingues',
              photo:
                'https://www.datocms-assets.com/2885/1590508642-carlos.png',
              title: 'IT Infrastructure Lead',
            },
            company: {
              name: 'Navi Capital',
              logo:
                'https://www.datocms-assets.com/2885/1590509560-navi-logo.png',
            },
          },
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
              title: 'Backend Engineer',
            },
            company: {
              name: 'Trivago',
              logo:
                'https://www.datocms-assets.com/2885/1582162145-trivago.svg',
            },
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
              title: 'Technical Director of Infrastructure',
            },
            company: {
              name: 'Roblox',
              logo:
                'https://www.datocms-assets.com/2885/1582180369-roblox-color.svg',
            },
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
              title: 'CTO',
            },
            company: {
              name: 'CircleCI',
              logo:
                'https://www.datocms-assets.com/2885/1582180745-circleci-logo.svg',
            },
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
              title: 'Engineer',
            },
            company: {
              name: 'imgix',
              logo: 'https://www.datocms-assets.com/2885/1582181250-imgix.svg',
            },
          },
          {
            quote:
              "I know many teams doing incredible work with Kubernetes but I also have heard horror stories about what happens when it doesn't go well. We attribute our systems' stability to the simplicity and elegance of Nomad.",
            caseStudyURL:
              'https://www.hashicorp.com/resources/betterhelp-s-hashicorp-nomad-use-case/',
            person: {
              firstName: 'Michael',
              lastName: 'Aldridge',
              photo:
                'https://www.datocms-assets.com/2885/1592925323-1587510032-michael-alridge.jpeg',
              title: 'Staff Systems Engineer',
            },
            company: {
              name: 'BetterHelp',
              logo:
                'https://www.datocms-assets.com/2885/1592925329-betterhelp-logo.png',
            },
          },
          {
            quote:
              "Nomad gives us a unified control plane, enabling hardware and driver rollouts using vendor's drivers - be it a centrifuge, incubator, or mass spectrometer.",
            caseStudyURL:
              'https://thenewstack.io/applying-workload-orchestration-to-experimental-biology/',
            person: {
              firstName: 'Dhasharath',
              lastName: 'Shrivathsa',
              photo:
                'https://www.datocms-assets.com/2885/1594233068-dharsharathshrivathsa.jpg',
              title: 'CEO',
            },
            company: {
              name: 'Radix',
              logo:
                'https://www.datocms-assets.com/2885/1594233325-radix-logo-1.svg',
            },
          },
          {
            quote:
              'Nomad has proven itself to be highly scalable, and we’re excited to scale our business alongside it.',
            caseStudyURL:
              'https://www.hashicorp.com/blog/how-nomad-powers-a-google-backed-indoor-farming-startup-to-disrupt-agtech/',
            person: {
              firstName: 'John',
              lastName: 'Spencer',
              photo:
                'https://www.datocms-assets.com/2885/1594236857-johnspencer.jpeg',
              title: 'Senior Site Reliability Engineer',
            },
            company: {
              name: 'Bowery',
              logo:
                'https://www.datocms-assets.com/2885/1594242826-bowery-logo-2.png',
            },
          },
        ]}
        featuredLogos={[
          {
            companyName: 'Trivago',
            url:
              'https://www.datocms-assets.com/2885/1582162317-trivago-monochromatic.svg',
          },
          {
            companyName: 'Roblox',
            url:
              'https://www.datocms-assets.com/2885/1582180373-roblox-monochrome.svg',
          },
          {
            companyName: 'CircleCI',
            url:
              'https://www.datocms-assets.com/2885/1582180745-circleci-logo.svg',
          },
          {
            companyName: 'SAP Ariba',
            url:
              'https://www.datocms-assets.com/2885/1580419436-logosap-ariba.svg',
          },
          {
            companyName: 'Pandora',
            url:
              'https://www.datocms-assets.com/2885/1523044075-pandora-black.svg',
          },
          {
            companyName: 'Citadel',
            url:
              'https://www.datocms-assets.com/2885/1582323352-logocitadelwhite-knockout.svg',
          },
          {
            companyName: 'Jet',
            url: 'https://www.datocms-assets.com/2885/1522341143-jet-black.svg',
          },
          {
            companyName: 'Deluxe',
            url:
              'https://www.datocms-assets.com/2885/1582323254-deluxe-logo.svg',
          },
          {
            companyName: 'Radix',
            url:
              'https://www.datocms-assets.com/2885/1594233325-radix-logo-1.svg',
          },
        ]}
      />

      <MiniCTA
        title="Are you using Nomad in production?"
        link={{
          text: 'Share your success story and receive special Nomad swag.',
          url: 'https://forms.gle/rdaLSuMGpvbomgYk9',
          type: 'outbound',
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
                url: require('./img/use-cases/simple-container-orchestration.svg'),
              },
              link: {
                external: false,
                title: 'Learn more',
                url: '/use-cases/simple-container-orchestration',
              },
            },
            {
              title: 'Non Containerized Application Orchestration',
              description:
                'Modernize non-containerized applications without rewrite.',
              image: {
                alt: null,
                format: 'png',
                url: require('./img/use-cases/non-containerized-application-orchestration.svg'),
              },
              link: {
                external: false,
                title: 'Learn more',
                url: '/use-cases/non-containerized-application-orchestration',
              },
            },
            {
              title: 'Automated Service Networking with Consul',
              description:
                'Service discovery and service mesh with HashiCorp Consul to ensure secure service-to-service communication.',
              image: {
                alt: null,
                format: 'png',
                url: require('./img/use-cases/automated-service-networking-with-consul.svg'),
              },
              link: {
                external: false,
                title: 'Learn more',
                url: '/use-cases/automated-service-networking-with-consul',
              },
            },
          ]}
        />
      </div>

      <LearnNomad
        items={[
          {
            title: 'Getting Started',
            category: 'Step-by-Step Guides',
            time: '24 mins',
            link: 'https://learn.hashicorp.com/collections/nomad/get-started',
            image: require('./img/learn-nomad/cap.svg'),
          },
          {
            title: 'Deploy and Manage Nomad Jobs',
            category: 'Step-by-Step Guides',
            time: '36 mins',
            link: 'https://learn.hashicorp.com/collections/nomad/manage-jobs',
            image: require('./img/learn-nomad/cubes.svg'),
          },
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
