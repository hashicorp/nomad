// The root folder for this documentation category is `pages/api-docs`
//
// - A string refers to the name of a file
// - A "category" value refers to the name of a directory
// - All directories must have an "index.mdx" file to serve as
//   the landing page for the category

export default [
  'index',
  '-----------',
  'libraries-and-sdks',
  'json-jobs',
  '-----------',
  'acl-policies',
  'acl-tokens',
  'agent',
  'allocations',
  'client',
  'deployments',
  'evaluations',
  'events',
  'jobs',
  'namespaces',
  'nodes',
  'metrics',
  {
    category: 'operator',
    content: ['autopilot' ,'raft', 'license', 'scheduler', 'snapshot'],
  },
  'plugins',
  'quotas',
  'recommendations',
  'regions',
  'scaling-policies',
  'search',
  'sentinel-policies',
  'status',
  'system',
  'ui',
  'validate',
  'volumes',
]
