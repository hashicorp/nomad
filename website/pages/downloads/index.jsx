import fetch from 'isomorphic-unfetch'
import VERSION from '../../data/version.js'
import ProductDownloader from '@hashicorp/react-product-downloader'
import Head from 'next/head'
import HashiHead from '@hashicorp/react-head'

export default function DownloadsPage({ downloadData }) {
  return (
    <div id="p-downloads" className="g-container">
      <HashiHead is={Head} title="Downloads | Nomad by HashiCorp" />
      <ProductDownloader
        product="Nomad"
        version={VERSION}
        downloads={downloadData}
        community="/resources"
      />
    </div>
  )
}

export async function unstable_getStaticProps() {
  return fetch(`https://releases.hashicorp.com/nomad/${VERSION}/index.json`)
    .then(r => r.json())
    .then(r => {
      // TODO: restructure product-downloader to run this logic internally
      return r.builds.reduce((acc, build) => {
        if (!acc[build.os]) acc[build.os] = {}
        acc[build.os][build.arch] = build.url
        return acc
      }, {})
    })
    .then(r => ({ props: { downloadData: r } }))
}
