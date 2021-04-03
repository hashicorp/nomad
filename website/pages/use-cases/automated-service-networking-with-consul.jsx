import UseCasesLayout from 'components/use-case-page'
import TextSplitWithImage from '@hashicorp/react-text-split-with-image'
import FeaturedSlider from '@hashicorp/react-featured-slider'

export default function AutomatedServiceNetworkingWithConsulPage() {
  return (
    <UseCasesLayout
      title="Automated Service Networking with Consul"
      description="Nomad natively integrates with Consul to provide automated clustering, built-in service discovery, and service mesh for secure service-to-service communications."
    >
      <TextSplitWithImage
        textSplit={{
          heading: 'Automatic Clustering',
          content:
            'Automatically bootstrap Nomad clusters using existing Consul agents on the same hosts.',
          textSide: 'right',
          links: [
            {
              text: 'Read More',
              url:
                'https://learn.hashicorp.com/tutorials/nomad/clustering#use-consul-to-automatically-cluster-nodes',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/auto-clustering-with-consul.svg'),
          alt: '',
        }}
      />

      <div className="with-border">
        <TextSplitWithImage
          textSplit={{
            heading: 'Automated Service Discovery',
            content:
              'Built-in service discovery, registration, and health check monitoring for all applications deployed under Nomad.',
            textSide: 'left',
            links: [
              {
                text: 'Read More',
                url: '/docs/integrations/consul-integration#service-discovery',
                type: 'inbound',
              },
            ],
          }}
          image={{
            url: require('./img/automated-service-discovery-with-consul.png'),
            alt: '',
          }}
        />
      </div>

      <TextSplitWithImage
        textSplit={{
          heading: 'Secure Service-to-Service Communication',
          content:
            'Enable seamless deployments of sidecar proxies and segmented microservices through Consul Connect.',
          textSide: 'right',
          links: [
            {
              text: 'Learn More',
              url: '/docs/integrations/consul-connect',
              type: 'inbound',
            },
          ],
        }}
        image={{
          url: require('./img/auto-service-to-service-communications.svg'),
          alt: '',
        }}
      />

      <FeaturedSlider
        heading="Case Studies"
        theme="dark"
        product="nomad"
        features={[
          {
            logo: {
              url:
                'https://www.datocms-assets.com/2885/1582161366-deluxe-logo.svg',
              alt: 'Deluxe',
            },
            image: {
              url: require('./img/deluxe.png'),
              alt: 'Deluxe Case Study',
            },
            heading: 'Deluxe',
            content:
              'Disrupt the traditional media supply chain with a digital platform powered by Nomad, Consul and Vault.',
            link: {
              text: 'Learn More',
              url:
                'https://www.hashicorp.com/resources/deluxe-hashistack-video-production',
              type: 'outbound',
            },
          },
          {
            logo: {
              url:
                'https://www.datocms-assets.com/2885/1582161581-seatgeek.svg',
              alt: 'SeatGeek',
            },
            image: {
              url: require('./img/seatgeek.png'),
              alt: 'Seat Geek Case Study',
            },
            heading: 'SeatGeek',
            content:
              'A team of 5 engineers built a infrastructure platform with Nomad, Consul, and Vault to provide tickets to millions of customers.',
            link: {
              text: 'Learn More',
              url:
                'https://www.hashicorp.com/resources/seatgeek-and-the-hashistack-a-tooling-and-automation-love-story',
              type: 'outbound',
            },
          },
        ]}
      />
    </UseCasesLayout>
  )
}
