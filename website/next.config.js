const withHashicorp = require('@hashicorp/platform-nextjs-plugin')
const redirects = require('./redirects')

module.exports = withHashicorp({
  dato: {
    // This token is safe to be in this public repository, it only has access to content that is publicly viewable on the website
    token: '88b4984480dad56295a8aadae6caad',
  },
  defaultLayout: true,
  nextOptimizedImages: true,
  transpileModules: ['@hashicorp/flight-icons'],
})({
  redirects() {
    return redirects
  },
  svgo: {
    plugins: [
      {
        removeViewBox: false,
      },
    ],
  },
  env: {
    HASHI_ENV: process.env.HASHI_ENV || 'development',
    SEGMENT_WRITE_KEY: 'qW11yxgipKMsKFKQUCpTVgQUYftYsJj0',
    BUGSNAG_CLIENT_KEY: '4fa712dfcabddd05da29fd1f5ea5a4c0',
    BUGSNAG_SERVER_KEY: '61141296f1ba00a95a8788b7871e1184',
    ENABLE_VERSIONED_DOCS: process.env.ENABLE_VERSIONED_DOCS || false,
  },
  images: {
    domains: ['www.datocms-assets.com'],
    disableStaticImages: true,
  },
})
