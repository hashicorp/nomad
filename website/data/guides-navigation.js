// The root folder for this documentation category is `pages/guides`
//
// - A string refers to the name of a file
// - A "category" value refers to the name of a directory
// - All directories must have an "index.mdx" file to serve as
//   the landing page for the category

export default [
  {
    category: 'install',
    content: [
      { category: 'quickstart' },
      {
        category: 'production',
        content: [
          'requirements',
          'nomad-agent',
          'reference-architecture',
          'deployment-guide'
        ]
      },
      'windows-service'
    ]
  },
  { category: 'upgrade', content: ['upgrade-specific'] },
  {
    category: 'integrations',
    content: ['consul-integration', 'consul-connect', 'vault-integration']
  },
  '-----------',
  {
    category: 'operating-a-job',
    content: [
      'configuring-tasks',
      'submitting-jobs',
      'inspecting-state',
      'accessing-logs',
      'resource-utilization',
      {
        category: 'update-strategies',
        content: [
          'rolling-upgrades',
          'blue-green-and-canary-deployments',
          'handling-signals'
        ]
      },
      {
        category: 'failure-handling-strategies',
        content: ['restart', 'check-restart', 'reschedule']
      },
      {
        category: 'advanced-scheduling',
        content: ['affinity', 'spread', 'preemption-service-batch']
      },
      { category: 'external', content: ['lxc'] }
    ]
  },
  {
    category: 'operations',
    content: [
      {
        category: 'cluster',
        content: ['manual', 'automatic', 'cloud_auto_join']
      },
      'federation',
      'node-draining',
      'outage',
      { category: 'monitoring-and-alerting', content: ['prometheus-metrics'] },
      'autopilot'
    ]
  },

  {
    category: 'security',
    content: ['encryption', 'acl', 'securing-nomad', 'vault-pki-integration']
  },
  { category: 'stateful-workloads' },
  {
    category: 'analytical-workloads',
    content: [
      {
        category: 'spark',
        content: [
          'pre',
          'customizing',
          'resource',
          'dynamic',
          'submit',
          'hdfs',
          'monitoring',
          'configuration'
        ]
      }
    ]
  },

  { category: 'load-balancing' },
  { category: 'governance-and-policy', content: [] },
  { category: 'web-ui', content: [] }
]
