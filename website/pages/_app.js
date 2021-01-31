import './style.css'
import '@hashicorp/nextjs-scripts/lib/nprogress/style.css'

import Router from 'next/router'
import Head from 'next/head'
import NProgress from '@hashicorp/nextjs-scripts/lib/nprogress'
import { ErrorBoundary } from '@hashicorp/nextjs-scripts/lib/bugsnag'
import createConsentManager from '@hashicorp/nextjs-scripts/lib/consent-manager'
import useAnchorLinkAnalytics from '@hashicorp/nextjs-scripts/lib/anchor-link-analytics'
import HashiStackMenu from '@hashicorp/react-hashi-stack-menu'
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

export default function App({ Component, pageProps }) {
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
      />
      {ALERT_BANNER_ACTIVE && (
        <AlertBanner {...alertBannerData} theme="nomad" />
      )}
      <HashiStackMenu />
      <ProductSubnav />
      <div className={`content${ALERT_BANNER_ACTIVE ? ' banner' : ''}`}>
        <Component {...pageProps} />
      </div>
      <Footer openConsentManager={openConsentManager} />
      <ConsentManager />
    </ErrorBoundary>
  )
}
