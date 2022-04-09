import VERSION from 'data/version'
import { productSlug } from 'data/metadata'
import ProductDownloadsPage from '@hashicorp/react-product-downloads-page'
import { generateStaticProps } from '@hashicorp/react-product-downloads-page/server'
import baseProps from 'components/downloads-props'
import s from './style.module.css'

export default function DownloadsPage(staticProps) {
  return <ProductDownloadsPage
    {...baseProps()}
    merchandisingSlot={
      <div className={s.releaseCandidate}>
        <p>
          A beta for Nomad v1.3.0 is available! The release can be{' '}
          <a href="https://releases.hashicorp.com/nomad/1.3.0-beta.1/">
          downloaded here.
          </a>
        </p>
      </div>
    }
    {...staticProps}
  />
}

export async function getStaticProps() {
  return generateStaticProps({
    product: productSlug,
    latestVersion: VERSION,
  })
}
