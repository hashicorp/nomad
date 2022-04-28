import VerticalTextBlockList from '@hashicorp/react-vertical-text-block-list'
import SectionHeader from '@hashicorp/react-section-header'
import Head from 'next/head'

export default function CommunityPage() {
  return (
    <div id="p-community">
      <Head>
        <title key="title">Community | Nomad by HashiCorp</title>
      </Head>
      <SectionHeader
        headline="Community"
        description="Nomad is an open-source project with a thriving community where active users are willing to help you via various mediums"
        use_h1={true}
      />
      <VerticalTextBlockList
        product="nomad"
        data={[
          {
            header: 'Community Forum',
            body:
              '<a href="https://discuss.hashicorp.com/c/nomad">Nomad Community Forum</a>',
          },
          {
            header: 'Office Hours',
            body:
              '<a href="https://www.hashicorp.com/community/office-hours">Ask a question</a> during community office hours',
          },
          {
            header: 'Announcement List',
            body:
              'High-priority, low-volume <a href="https://groups.google.com/g/hashicorp-announce">announcements about HashiCorp products</a>, including release information and security bulletins.',
          },
          {
            header: 'Bug Tracker',
            body:
              '<a href="https://github.com/hashicorp/nomad/issues">Issue tracker on GitHub</a>. Please only use this for reporting bugs. Do not ask for general help here; use the <a href="https://discuss.hashicorp.com/c/nomad">Community Forum</a> or the mailing list for that.',
          },
          {
            header: 'Webinars',
            body:
              '<a href="https://www.hashicorp.com/events?product=nomad&type=all">Register for webinars</a> or <a href="https://www.hashicorp.com/events/webinars/recorded?product=nomad&type=all">watch recorded webinars</a>.',
          },
        ]}
      />
    </div>
  )
}
