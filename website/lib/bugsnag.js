import React from 'react'
import bugsnag from '@bugsnag/js'
import bugsnagReact from '@bugsnag/plugin-react'

const apiKey =
  typeof window === 'undefined'
    ? '61141296f1ba00a95a8788b7871e1184'
    : '4fa712dfcabddd05da29fd1f5ea5a4c0'

const bugsnagClient = bugsnag({
  apiKey,
  releaseStage: process.env.NODE_ENV || 'development'
})

bugsnagClient.use(bugsnagReact, React)

export default bugsnagClient
