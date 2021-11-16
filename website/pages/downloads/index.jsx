import VERSION from 'data/version'
import { productSlug } from 'data/metadata'
import ProductDownloadsPage from '@hashicorp/react-product-downloads-page'
import { generateStaticProps } from '@hashicorp/react-product-downloads-page/server'
import s from './style.module.css'

export default function DownloadsPage(staticProps) {
  return (
    <ProductDownloadsPage
      getStartedDescription="Follow step-by-step tutorials on the essentials of Nomad."
      getStartedLinks={[
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
      ]}
      logo={
        <img
          className={s.logo}
          alt="Nomad"
          src={require('@hashicorp/mktg-logos/product/nomad/primary/color.svg')}
        />
      }
      tutorialLink={{
        href: 'https://learn.hashicorp.com/nomad',
        label: 'View Tutorials at HashiCorp Learn',
      }}
      {...staticProps}
    />
  )
}

export async function getStaticProps() {
  return generateStaticProps({
    product: productSlug,
    latestVersion: VERSION,
  })
}
