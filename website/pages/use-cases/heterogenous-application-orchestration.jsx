import UseCasesLayout from 'components/use-case-page'
import TextSplitWithImage from '@hashicorp/react-text-split-with-image'
import FeaturedSlider from '@hashicorp/react-featured-slider'

export default function HeterogenousApplicationOrchestrationPage() {
  return (
    <UseCasesLayout
      title="Nomad embraces the Post-Container World"
      description="Nomad allows you to write and ship your business logic, not YAML config and Dockerfiles."
    >
      <TextSplitWithImage
        textSplit={{
          heading: 'Run thousands of tasks per node',
          content: 'Improve resource utilization and reduce costs by shipping isolates, not clunky containers. This allows operators to more densly pack their business logic onto nodes and drastically cut cost and architectural complexity.',
          textSide: 'left',
        }}
        image={{
          url: require('./img/isolates.png'),
          alt: 'Bin Packing',
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Simplify Developer Experience',
          content: 'Developers can simply compile code to WASM and ship the binary. No Dockerfile wrangling necessary. No Golden AMIs needed. Just build and ship.',
          textSide: 'right',
          links: [
            {
               text: 'Read more',
              url: 'https://learn.hashicorp.com/collections/nomad/job-updates',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/devex.png'),
          alt: '',
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Orchestrate Anything',
          content: 'Rust on WASM? Raw Golang binaries? Docker Containers? JAR Files? Firecracker VMs? Podman containers? QEMU VMs? No problem. Nomad can orchestrate and connect tasks with any type of artifact.' ,
          textSide: 'left',
        }}
        image={{
          url: require('./img/everything.png'),
          alt: 'Non-Containerized Orchestration',
        }}
      />

      <FeaturedSlider
        heading="The Nomad Challenges"
        theme="dark"
        features={[
          {
            logo: {
              url:
                'https://www.datocms-assets.com/2885/1620155099-brandhcnomadprimaryattributedcolorwhite.svg',
              alt: 'GrayMeta',
            },
            image: {
              url: require('./img/c2m.png'),
              alt: 'The Nomad Challenges Presentation',
            },
            heading: 'The Nomad Challenges',
            content:
              'See how Nomad scaled to 50 Million WASM tasks and 2 Million containers deployed across the globe in the Nomad Challenges.',
            link: {
              text: 'Watch Presentation',
              url:
                'https://www.hashicorp.com/resources/backend-batch-processing-nomad',
              type: 'outbound',
            },
          },
        ]}
      />
    </UseCasesLayout>
  )
}
