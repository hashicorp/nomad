const withHashicorp = require('@hashicorp/nextjs-scripts')
const path = require('path')
const dotenv = require('dotenv')

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
    rewrites: () => [
      {
        source: '/api/:path*',
        destination: '/api-docs/:path*',
      },
    ],
  },
  env: {
    HASHI_ENV: process.env.HASHI_ENV || 'development',
    SEGMENT_WRITE_KEY: 'qW11yxgipKMsKFKQUCpTVgQUYftYsJj0',
    BUGSNAG_CLIENT_KEY: '4fa712dfcabddd05da29fd1f5ea5a4c0',
    BUGSNAG_SERVER_KEY: '61141296f1ba00a95a8788b7871e1184',
  },
})
