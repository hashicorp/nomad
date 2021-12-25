import s from '../../pages/downloads/style.module.css'

export default function DownloadsProps(preMerchandisingSlot) {
  return {
    getStartedDescription:
      'Follow step-by-step tutorials on the essentials of Nomad.',
    getStartedLinks: [
      {
        label: 'Getting Started',
        href: 'https://learn.hashicorp.com/collections/nomad/get-started',
      },
      {
        label: 'Deploy and Manage Nomad Jobs',
        href: 'https://learn.hashicorp.com/collections/nomad/manage-jobs',
      },
      {
        label: 'Explore the Nomad Web UI',
        href: 'https://learn.hashicorp.com/collections/nomad/web-ui',
      },
      {
        label: 'View all Nomad tutorials',
        href: 'https://learn.hashicorp.com/nomad',
      },
    ],
    logo: (
      <img
        className={s.logo}
        alt="Nomad"
        src={require('@hashicorp/mktg-logos/product/nomad/primary/color.svg')}
      />
    ),
    tutorialLink: {
      href: 'https://learn.hashicorp.com/nomad',
      label: 'View Tutorials at HashiCorp Learn',
    },
    merchandisingSlot: (
      <>
        {preMerchandisingSlot && preMerchandisingSlot}
        <div className={s.releaseCandidate}>
          <p>
            A release candidate for Nomad v1.2.0 is available! The release can
            be{' '}
            <a href="https://releases.hashicorp.com/nomad/1.2.0-rc1/">
              downloaded here.
            </a>
          </p>
        </div>
      </>
    ),
  }
}
