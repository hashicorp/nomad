import './style.css'
import '@hashicorp/nextjs-scripts/lib/nprogress/style.css'

import Router from 'next/router'
import Head from 'next/head'
import NProgress from '@hashicorp/nextjs-scripts/lib/nprogress'
import { ErrorBoundary } from '@hashicorp/nextjs-scripts/lib/bugsnag'
import createConsentManager from '@hashicorp/nextjs-scripts/lib/consent-manager'
import useAnchorLinkAnalytics from '@hashicorp/nextjs-scripts/lib/anchor-link-analytics'
import MegaNav from '@hashicorp/react-mega-nav'
import AlertBanner from '@hashicorp/react-alert-banner'
import HashiHead from '@hashicorp/react-head'
import Footer from 'components/footer'
import ProductSubnav from 'components/subnav'
import Error from './_error'
import alertBannerData, { ALERT_BANNER_ACTIVE } from 'data/alert-banner'

NProgress({ Router })
const { ConsentManager, openConsentManager } = createConsentManager({
  preset: 'oss',
})

function App({ Component, pageProps }) {
  useAnchorLinkAnalytics()

  return (
    <ErrorBoundary FallbackComponent={Error}>
      <HashiHead
        is={Head}
        title="Nomad by HashiCorp"
        siteName="Nomad by HashiCorp"
        description="Nomad is a highly available, distributed, data-center aware cluster and application scheduler designed to support the modern datacenter with support for long-running services, batch jobs, and much more."
        image="https://www.nomadproject.io/img/og-image.png"
        icon={[{ href: '/favicon.ico' }]}
        preload={[
          { href: '/fonts/klavika/medium.woff2', as: 'font' },
          { href: '/fonts/gilmer/light.woff2', as: 'font' },
          { href: '/fonts/gilmer/regular.woff2', as: 'font' },
          { href: '/fonts/gilmer/medium.woff2', as: 'font' },
          { href: '/fonts/gilmer/bold.woff2', as: 'font' },
          { href: '/fonts/metro-sans/book.woff2', as: 'font' },
          { href: '/fonts/metro-sans/regular.woff2', as: 'font' },
          { href: '/fonts/metro-sans/semi-bold.woff2', as: 'font' },
          { href: '/fonts/metro-sans/bold.woff2', as: 'font' },
          { href: '/fonts/dejavu/mono.woff2', as: 'font' },
        ]}
      />
      {ALERT_BANNER_ACTIVE && (
        <AlertBanner {...alertBannerData} theme="nomad" />
      )}
      <MegaNav product="Nomad" />
      <ProductSubnav />
      <div className={`content${ALERT_BANNER_ACTIVE ? ' banner' : ''}`}>
        <Component {...pageProps} />
      </div>
      <Footer openConsentManager={openConsentManager} />
      <ConsentManager />
    </ErrorBoundary>
  )
}

App.getInitialProps = async ({ Component, ctx }) => {
  let pageProps = {}

  if (Component.getInitialProps) {
    pageProps = await Component.getInitialProps(ctx)
  } else if (Component.isMDXComponent) {
    // fix for https://github.com/mdx-js/mdx/issues/382
    const mdxLayoutComponent = Component({}).props.originalType
    if (mdxLayoutComponent.getInitialProps) {
      pageProps = await mdxLayoutComponent.getInitialProps(ctx)
    }
  }

  return { pageProps }
}

export default App
