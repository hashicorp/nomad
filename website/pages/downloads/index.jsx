import VERSION from 'data/version.js'
import ProductDownloader from '@hashicorp/react-product-downloader'
import Head from 'next/head'
import HashiHead from '@hashicorp/react-head'

export default function DownloadsPage({ releaseData }) {
  return (
    <div id="p-downloads" className="g-container">
      <HashiHead is={Head} title="Downloads | Nomad by HashiCorp" />
      <ProductDownloader
        product="Nomad"
        version={VERSION}
        releaseData={releaseData}
        community="/resources"
      />
    </div>
  )
}

export async function getStaticProps() {
  return fetch(`https://releases.hashicorp.com/nomad/${VERSION}/index.json`)
    .then((res) => res.json())
    .then((releaseData) => ({ props: { releaseData } }))
    .catch(() => {
      throw new Error(
        `--------------------------------------------------------
        Unable to resolve version ${VERSION} on releases.hashicorp.com from link
        <https://releases.hashicorp.com/nomad/${VERSION}/index.json>. Usually this
        means that the specified version has not yet been released. The downloads page
        version can only be updated after the new version has been released, to ensure
        that it works for all users.
        ----------------------------------------------------------`
      )
    })
}
