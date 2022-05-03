import './style.css'
import '@hashicorp/platform-util/nprogress/style.css'

import Router from 'next/router'
import Head from 'next/head'
import rivetQuery from '@hashicorp/nextjs-scripts/dato/client'
import NProgress from '@hashicorp/platform-util/nprogress'
import { ErrorBoundary } from '@hashicorp/platform-runtime-error-monitoring'
import createConsentManager from '@hashicorp/react-consent-manager/loader'
import localConsentManagerServices from 'lib/consent-manager-services'
import useFathomAnalytics from '@hashicorp/platform-analytics'
import useAnchorLinkAnalytics from '@hashicorp/platform-util/anchor-link-analytics'
import HashiStackMenu from '@hashicorp/react-hashi-stack-menu'
import AlertBanner from '@hashicorp/react-alert-banner'
import HashiHead from '@hashicorp/react-head'
import ProductSubnav from 'components/subnav'
import Error from './_error'
import alertBannerData, { ALERT_BANNER_ACTIVE } from 'data/alert-banner'
import StandardLayout from 'layouts/standard'

NProgress({ Router })
const { ConsentManager } = createConsentManager({
  preset: 'oss',
  otherServices: [...localConsentManagerServices],
})

export default function App({ Component, pageProps, layoutData }) {
  useFathomAnalytics()
  useAnchorLinkAnalytics()

  const Layout = Component.layout ?? StandardLayout

  return (
    <ErrorBoundary FallbackComponent={Error}>
      <HashiHead
        is={Head}
        title="Nomad by HashiCorp"
        siteName="Nomad by HashiCorp"
        description="Nomad is a highly available, distributed, data-center aware cluster and application scheduler designed to support the modern datacenter with support for long-running services, batch jobs, and much more."
        image="https://www.nomadproject.io/img/og-image.png"
        icon={[{ href: '/_favicon.ico' }]}
      />
      {ALERT_BANNER_ACTIVE && (
        <AlertBanner {...alertBannerData} product="nomad" hideOnMobile />
      )}
      <Layout {...(layoutData && { data: layoutData })}>
        <div className={`content${ALERT_BANNER_ACTIVE ? ' banner' : ''}`}>
          <Component {...pageProps} />
        </div>
      </Layout>
      <ConsentManager />
    </ErrorBoundary>
  )
}

App.getInitialProps = async ({ Component, ctx }) => {
  const layoutQuery = Component.layout
    ? Component.layout?.rivetParams ?? null
    : StandardLayout.rivetParams

  const layoutData = layoutQuery ? await rivetQuery(layoutQuery) : null

  let pageProps = {}

  if (Component.getInitialProps) {
    pageProps = await Component.getInitialProps(ctx)
  }
  return { pageProps, layoutData }
}
