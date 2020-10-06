const withHashicorp = require('@hashicorp/nextjs-scripts')
const path = require('path')
const dotenv = require('dotenv')
//const redirects = require('./redirects')
const rewrites = require('./rewrites')

dotenv.config()

module.exports = withHashicorp({
  defaultLayout: true,
  transpileModules: ['is-absolute-url', '@hashicorp/react-mega-nav'],
  mdx: { resolveIncludes: path.join(__dirname, 'pages/partials') },
})({
  svgo: {
    plugins: [
      {
        removeViewBox: false,
      },
    ],
  },
  experimental: {
    modern: true,
    redirects() {
      return [
        {
          source: '/docs/agent',
          destination: '/docs/install/production/nomad-agent/',
        },
      ]
    },
    rewrites() {
      return rewrites
    },
  },
  env: {
    HASHI_ENV: process.env.HASHI_ENV || 'development',
    SEGMENT_WRITE_KEY: 'qW11yxgipKMsKFKQUCpTVgQUYftYsJj0',
    BUGSNAG_CLIENT_KEY: '4fa712dfcabddd05da29fd1f5ea5a4c0',
    BUGSNAG_SERVER_KEY: '61141296f1ba00a95a8788b7871e1184',
  },
})
