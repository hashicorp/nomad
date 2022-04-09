import UseCasesLayout from 'components/use-case-page'
import TextSplitWithImage from '@hashicorp/react-text-split-with-image'
import FeaturedSlider from '@hashicorp/react-featured-slider'

export default function NonContainerizedApplicationOrchestrationPage() {
  return (
    <UseCasesLayout
      title="Non-Containerized Application Orchestration"
      description="Nomad's flexible workload support enables an organization to run containerized, non containerized, and batch applications through a single workflow. Nomad brings core orchestration benefits to legacy applications without needing to containerize via pluggable task drivers."
    >
      <TextSplitWithImage
        textSplit={{
          heading: 'Non-Containerized Orchestration',
          content:
            'Deploy, manage, and scale your non-containerized applications using the Java, QEMU, or exec drivers.',
          textSide: 'right',
          links: [
            {
              text: 'Watch the Webinar',
              url:
                'https://www.hashicorp.com/resources/move-your-vmware-workloads-nomad',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/non-containerized-orch.png'),
          alt: 'Non-Containerized Orchestration',
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Improve Resource Utilization with Bin Packing',
          content:
            'Improve resource utilization and reduce costs for non-containerized applications through Nomad’s bin-packing placements.',
          textSide: 'left',
        }}
        image={{
          url: require('./img/resource-utilization.png'),
          alt: 'Bin Packing',
        }}
      />

      <TextSplitWithImage
        textSplit={{
          heading: 'Zero Downtime Deployments',
          content:
            'Apply modern upgrade strategies for legacy applications through rolling updates, blue/green, or canary deployment strategies.',
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
          url: require('./img/zero-downtime.png'),
          alt: '',
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
          url: require('./img/run-on-prem-with-ease.png'),
          alt: '',
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
              text: 'Watch GrayMeta tech presentation',
              url:
                'https://www.hashicorp.com/resources/backend-batch-processing-nomad',
              type: 'outbound',
            },
          ],
        }}
        image={{
          url: require('./img/batch-workloads@3x.png'),
          alt: '',
        }}
      />

      <FeaturedSlider
        heading="Case Study"
        theme="dark"
        features={[
          {
            logo: {
              url:
                'https://www.datocms-assets.com/2885/1582149907-graymetalogo.svg',
              alt: 'GrayMeta',
            },
            image: {
              url: require('./img/grey_meta.png'),
              alt: 'GrayMeta Presentation',
            },
            heading: 'GrayMeta',
            content:
              'Move an application from a traditional model of processing jobs out of a queue to scheduling them as container jobs in Nomad.',
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
