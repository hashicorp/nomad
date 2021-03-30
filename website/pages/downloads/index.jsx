import VERSION, { packageManagers } from 'data/version.js'
import ProductDownloader from '@hashicorp/react-product-downloader'
import Head from 'next/head'
import HashiHead from '@hashicorp/react-head'
import { productName, productSlug } from 'data/metadata'
import s from './style.module.css'

export default function DownloadsPage({ releases }) {
  return (
    <div className={s.root}>
      <HashiHead is={Head} title="Downloads | Nomad by HashiCorp" />
      <ProductDownloader
        releases={releases}
        packageManagers={packageManagers}
        productName={productName}
        productId={productSlug}
        latestVersion={VERSION}
        getStartedDescription="Follow step-by-step tutorials on the essentials of Nomad."
        getStartedLinks={[
          {
            label: 'Get Started With Nomad',
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
            src={require('./img/nomad-logo.svg')}
          />
        }
        product="nomad"
        tutorialLink={{
          href: 'https://learn.hashicorp.com/nomad',
          label: 'View Tutorials at HashiCorp Learn',
        }}
      />
    </div>
  )
}

export async function getStaticProps() {
  return fetch(`https://releases.hashicorp.com/nomad/index.json`, {
    headers: {
      'Cache-Control': 'no-cache',
    },
  })
    .then((res) => res.json())
    .then((result) => {
      return {
        props: {
          releases: result,
        },
      }
    })
    .catch(() => {
      throw new Error(
        `--------------------------------------------------------
        Unable to resolve version ${VERSION} on releases.hashicorp.com from link
        <https://releases.hashicorp.com/${productSlug}/${VERSION}/index.json>. Usually this
        means that the specified version has not yet been released. The downloads page
        version can only be updated after the new version has been released, to ensure
        that it works for all users.
        ----------------------------------------------------------`
      )
    })
}
