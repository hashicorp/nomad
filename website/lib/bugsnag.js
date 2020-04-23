import React from 'react'
import Bugsnag from '@bugsnag/js'
import BugsnagReact from '@bugsnag/plugin-react'

const apiKey =
  typeof window === 'undefined'
    ? '61141296f1ba00a95a8788b7871e1184' // server key
    : '4fa712dfcabddd05da29fd1f5ea5a4c0' // client key

if (!Bugsnag._client) {
  Bugsnag.start({
    apiKey,
    plugins: [new BugsnagReact(React)],
    otherOptions: { releaseStage: process.env.NODE_ENV || 'development' },
  })
}

export default Bugsnag
