/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/**
 * Define your custom redirects within this file.
 *
 * Vercel's redirect documentation:
 * https://nextjs.org/docs/api-reference/next.config.js/redirects
 *
 * Relative paths with fragments (#) are not supported.
 * For destinations with fragments, use an absolute URL.
 *
 * Playground for testing url pattern matching: https://npm.runkit.com/path-to-regexp
 *
 * Note that redirects defined in a product's redirects file are applied to
 * the developer.hashicorp.com domain, which is where the documentation content
 * is rendered. Redirect sources should be prefixed with the product slug
 * to ensure they are scoped to the product's section. Any redirects that are
 * not prefixed with a product slug will be ignored.
 */
module.exports = [
  /*
  Example redirect:
  {
    source: '/nomad/docs/internal-docs/my-page',
    destination: '/nomad/docs/internals/my-page',
    permanent: true,
  },
  */

  /**
   * /s/* redirects for useful links that need a stable URL but we may need to
   * change its destination in the future.
   */
  {
    source: '/nomad/s/envoy-bootstrap-error',
    destination:
      'https://developer.hashicorp.com/nomad/docs/networking/consul/service-mesh#troubleshooting',
    permanent: false,
  },
  {
    source: '/nomad/s/vault-workload-identity-migration',
    destination:
      'https://developer.hashicorp.com/nomad/docs/v1.8.x/integrations/vault/acl#migrating-to-using-workload-identity-with-vault',
    permanent: false,
  },
  {
    source: '/nomad/tools/autoscaling/internals/:path*',
    destination: '/nomad/tools/autoscaling/concepts/:path*',
    permanent: true,
  },
  {
    source: '/nomad/tools/autoscaling/concepts/checks',
    destination: '/nomad/tools/autoscaling/concepts/policy-eval/checks',
    permanent: true,
  },
  {
    source: '/nomad/tools/autoscaling/concepts/node-selector-strategy',
    destination:
      '/nomad/tools/autoscaling/concepts/policy-eval/node-selector-strategy',
    permanent: true,
  },
  {
    source: '/nomad/docs/integrations/vault-integration',
    destination: '/nomad/docs/integrations/vault',
    permanent: true,
  },
  {
    source: '/nomad/docs/integrations/consul-integration',
    destination: '/nomad/docs/networking/consul',
    permanent: true,
  },
  {
    source: '/nomad/docs/integrations/consul-connect',
    destination: '/nomad/docs/networking/consul/service-mesh',
    permanent: true,
  },
  {
    source: '/nomad/tools/autoscaling/agent/source',
    destination: '/nomad/tools/autoscaling/agent/policy',
    permanent: true,
  },
  {
    source: '/nomad/plugins/drivers/remote/:slug*',
    destination: 'nomad/plugins/drivers/',
    permanent: true,
  },
  {
    source: '/nomad/plugins/drivers/community/lxc',
    destination: '/nomad/plugins/drivers/community/',
    permanent: true,
  },
  // CSI plugins moved under new storage path alongside new host volume plugins
  {
    source: '/nomad/docs/concepts/plugins/csi',
    destination: '/nomad/docs/concepts/plugins/storage/csi',
    permanent: true,
  },
  {
    source: '/nomad/plugins/drivers/virt/client',
    destination: '/nomad/plugins/drivers/virt/install',
    permanent: true,
  },
  {
    source: '/nomad/plugins/drivers/virt/installation',
    destination: '/nomad/plugins/drivers/virt/install',
    permanent: true,
  },
  {
    source: '/nomad/docs/devices',
    destination: '/nomad/plugins/devices/',
    permanent: true,
  },
  {
    source: '/nomad/plugins/devices/community',
    destination: '/nomad/plugins/devices/',
    permanent: true,
  },
  {
    source: '/nomad/plugins/devices/community/usb',
    destination: '/nomad/plugins/devices/usb',
    permanent: true,
  },
  {
    source: '/nomad/drivers/community/containerd',
    destination: 'https://github.com/Roblox/nomad-driver-containerd',
    permanent: true,
  },
  {
    source: '/nomad/drivers/community/pledge',
    destination: 'https://github.com/shoenig/nomad-pledge-driver',
    permanent: true,
  },
  {
    source: '/nomad/drivers/community/firecracker-task-driver',
    destination: 'https://github.com/cneira/firecracker-task-driver',
    permanent: true,
  },
  {
    source: '/nomad/drivers/community/jail-task-driver',
    destination: 'https://github.com/cneira/jail-task-driver',
    permanent: true,
  },
  {
    source: '/nomad/drivers/community/lightrun',
    destination: 'https://docs.lightrun.com/integrations/nomad/',
    permanent: true,
  },
  {
    source: '/nomad/drivers/community/pot',
    destination: 'https://github.com/bsdpot/nomad-pot-driver',
    permanent: true,
  },
  {
    source: '/nomad/drivers/community/rookout',
    destination: 'https://github.com/Rookout/rookout-nomad-driver',
    permanent: true,
  },
  {
    source: '/nomad/drivers/community/singularity',
    destination: 'https://github.com/hpcng/nomad-driver-singularity',
    permanent: true,
  },
  {
    source: '/nomad/drivers/community/nspawn',
    destination: 'https://github.com/JanMa/nomad-driver-nspawn',
    permanent: true,
  },
  {
    source: '/nomad/drivers/community/iis',
    destination: 'https://github.com/Roblox/nomad-driver-iis',
    permanent: true,
  },
  {
    source: '/nomad/drivers/community/nomad-iis',
    destination: 'https://nomad-iis.sevensolutions.cc/',
    permanent: true,
  },
  /**
   * Nomad IA redirects
   */
  {
    source: '/nomad/docs/commands/:path*',
    destination: '/nomad/commands/:path*',
    permanent: true,
  },
  {
    source: '/nomad/intro',
    destination: '/nomad/docs/what-is-nomad',
    permanent: true,
  },
  {
    source: '/nomad/intro/use-cases',
    destination: '/nomad/docs/use-cases',
    permanent: true,
  },
  {
    source: '/nomad/intro/vs/:path*',
    destination: '/nomad/docs/what-is-nomad',
    permanent: true,
  },
  {
    source: '/nomad/docs/nomad-vs-kubernetes/:path*',
    destination: '/nomad/docs/what-is-nomad',
    permanent: true,
  },
  {
    source: '/nomad/docs/install/quickstart',
    destination: '/nomad/docs/quickstart',
    permanent: true,
  },
  {
    source: '/nomad/docs/install/windows-service',
    destination: '/nomad/docs/deploy/production/windows-service',
    permanent: true,
  },
  {
    source: '/nomad/docs/install/production/:path*',
    destination: '/nomad/docs/deploy/production/:path*',
    permanent: true,
  },
  {
    source: '/nomad/docs/concepts/architecture',
    destination: '/nomad/docs/architecture',
    permanent: true,
  },
  {
    source: '/nomad/docs/concepts/architecture/federation',
    destination: '/nomad/docs/architecture/cluster/federation',
    permanent: true,
  },
  {
    source: '/nomad/docs/concepts/acl',
    destination: '/nomad/docs/secure/acl',
    permanent: true,
  },
  {
    source: '/nomad/docs/architecture/acl',
    destination: '/nomad/docs/secure/acl',
    permanent: true,
  },
  {
    source: '/nomad/docs/concepts/consensus',
    destination: '/nomad/docs/architecture/cluster/consensus',
    permanent: true,
  },
  {
    source: '/nomad/docs/concepts/cpu',
    destination: '/nomad/docs/architecture/cpu',
    permanent: true,
  },
  {
    source: '/nomad/docs/concepts/gossip',
    destination: '/nomad/docs/architecture/security/gossip',
    permanent: true,
  },
  {
    source: '/nomad/docs/concepts/node-pools',
    destination: '/nomad/docs/architecture/cluster/node-pools',
    permanent: true,
  },
  {
    source: '/nomad/docs/concepts/security',
    destination: '/nomad/docs/architecture/security',
    permanent: true,
  },
  {
    source: '/nomad/docs/operations/stateful-workloads',
    destination: '/nomad/docs/architecture/storage/stateful-workloads',
    permanent: true,
  },
  {
    source: '/nomad/docs/concepts/plugins/storage/:path*',
    destination: '/nomad/docs/architecture/storage/:path*',
    permanent: true,
  },
  {
    source: '/nomad/docs/job-specification/hcl2/:path*',
    destination: '/nomad/docs/reference/hcl2/:path*',
    permanent: true,
  },
  {
    source: '/nomad/docs/operations/metrics-reference',
    destination: '/nomad/docs/reference/metrics',
    permanent: true,
  },
  {
    source: '/nomad/docs/runtime',
    destination: '/nomad/docs/reference/runtime-environment-settings',
    permanent: true,
  },
  {
    source: '/nomad/docs/runtime/environment',
    destination: '/nomad/docs/reference/runtime-environment-settings',
    permanent: true,
  },
  {
    source: '/nomad/docs/runtime/interpolation',
    destination: '/nomad/docs/reference/runtime-variable-interpolation',
    permanent: true,
  },
  {
    source: '/nomad/docs/enterprise/sentinel',
    destination: '/nomad/docs/reference/sentinel-policy',
    permanent: true,
  },
  {
    source: '/nomad/docs/drivers/:path*',
    destination: '/nomad/docs/deploy/task-driver/:path*',
    permanent: true,
  },
  {
    source: '/nomad/docs/operations/nomad-agent',
    destination: '/nomad/docs/deploy/nomad-agent',
    permanent: true,
  },
  {
    source: '/nomad/docs/operations/federation',
    destination: '/nomad/docs/deploy/clusters/federation-considerations',
    permanent: true,
  },
  {
    source: '/nomad/docs/operations/federation/failure',
    destination: '/nomad/docs/deploy/clusters/federation-failure-scenarios',
    permanent: true,
  },
  {
    source: '/nomad/docs/operations/garbage-collection',
    destination: '/nomad/docs/manage/garbage-collection',
    permanent: true,
  },
  {
    source: '/nomad/docs/operations/key-management',
    destination: '/nomad/docs/manage/key-management',
    permanent: true,
  },
  {
    source: '/nomad/docs/operations/benchmarking',
    destination: '/nomad/docs/scale/benchmarking',
    permanent: true,
  },
  {
    source: '/nomad/docs/concepts/scheduling/scheduling',
    destination: '/nomad/docs/concepts/scheduling/how-scheduling-works',
    permanent: true,
  },
  {
    source: '/nomad/docs/schedulers',
    destination: '/nomad/docs/concepts/scheduling/schedulers',
    permanent: true,
  },
  {
    source: '/nomad/docs/operations/ipv6-support',
    destination: '/nomad/docs/networking/ipv6',
    permanent: true,
  },
  {
    source: '/nomad/docs/operations/monitoring-nomad',
    destination: '/nomad/docs/monitor',
    permanent: true,
  },
  {
    source: '/nomad/docs/concepts/acl/auth-methods/oidc',
    destination: '/nomad/docs/secure/authentication/oidc',
    permanent: true,
  },
  {
    source: '/nomad/docs/concepts/acl/auth-methods/jwt',
    destination: '/nomad/docs/secure/authentication/jwt',
    permanent: true,
  },
  {
    source: '/nomad/docs/operations/aws-oidc-provider',
    destination: '/nomad/docs/secure/workload-identity/aws-oidc-provider',
    permanent: true,
  },
  {
    source: '/nomad/who-uses-nomad',
    destination: '/nomad/use-cases',
    permanent: true,
  },
  {
    source: '/nomad/docs/networking/service-mesh',
    destination: '/nomad/docs/networking/consul/service-mesh',
    permanent: true,
  },
  {
    source: '/nomad/docs/integrations',
    destination: '/nomad/docs/networking/consul',
    permanent: true,
  },
  {
    source: '/nomad/docs/integrations/consul',
    destination: '/nomad/docs/networking/consul',
    permanent: true,
  },
  {
    source: '/nomad/docs/integrations/consul/acl',
    destination: '/nomad/docs/secure/acl/consul',
    permanent: true,
  },
  {
    source: '/nomad/docs/integrations/consul/service-mesh',
    destination: '/nomad/docs/networking/consul/service-mesh',
    permanent: true,
  },
  {
    source: '/nomad/docs/integrations/vault/:path*',
    destination: '/nomad/docs/secure/vault/:path*',
    permanent: true,
  },
  // section index pages no longer in existence
  {
    source: '/nomad/docs/operations',
    destination: '/nomad/docs',
    permanent: true,
  },
  {
    source: '/nomad/docs/integrations',
    destination: '/nomad/docs',
    permanent: true,
  },
  {
    source: '/nomad/docs/concepts',
    destination: '/nomad/docs',
    permanent: true,
  },
  /* redirect to handle new /commands path in 1.9, 1.8 */
  {
    source: '/nomad/docs/:version(v1.(?:8|9).x)/commands/:path*',
    destination: '/nomad/commands/:version/:path*',
    permanent: true,
  },
  /* redirects for versioned docs in 1.9, 1.8 */
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/what-is-nomad/',
    destination: '/nomad/docs/what-is-nomad',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/quickstart/',
    destination: '/nomad/docs/:version/install/quickstart',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/architecture/',
    destination: '/nomad/docs/:version/concepts/architecture',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/architecture/cluster/consensus/',
    destination: '/nomad/docs/:version/concepts/consensus',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/architecture/cluster/federation/',
    destination: '/nomad/docs/:version/concepts/architecture/federation',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/architecture/cluster/node-pools/',
    destination: '/nomad/docs/:version/concepts/node-pools',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/architecture/cpu/',
    destination: '/nomad/docs/:version/concepts/cpu',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/architecture/security/',
    destination: '/nomad/docs/:version/concepts/security',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/architecture/security/gossip/',
    destination: '/nomad/docs/:version/concepts/gossip',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/architecture/storage/:slug*',
    destination: '/nomad/docs/:version/concepts/plugins/storage/:slug*',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/concepts/scheduling/how-scheduling-works/',
    destination: '/nomad/docs/:version/concepts/scheduling/scheduling',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/concepts/scheduling/schedulers/',
    destination: '/nomad/docs/:version/schedulers',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/intro/use-cases/',
    destination: '/nomad/docs/use-cases',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/deploy/:slug*',
    destination: '/nomad/docs/:version/install/:slug*',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/deploy/production/windows-service/',
    destination: '/nomad/docs/:version/install/windows-service',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/deploy/nomad-agent/',
    destination: '/nomad/docs/:version/operations/nomad-agent',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/deploy/clusters/federation-considerations/',
    destination: '/nomad/docs/:version/operations/federation',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/deploy/clusters/federation-failure-scenarios/',
    destination: '/nomad/docs/:version/operations/federation/failure',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/deploy/task-driver/:slug*',
    destination: '/nomad/docs/:version/drivers/:slug*',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/networking/consul/',
    destination: '/nomad/docs/:version/integrations',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/manage/garbage-collection/',
    destination: '/nomad/docs/:version/operations/garbage-collection',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/manage/key-management/',
    destination: '/nomad/docs/:version/operations/key-management',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/monitor/',
    destination: '/nomad/docs/:version/operations/monitoring-nomad',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/networking/consul/service-mesh/',
    destination: '/nomad/docs/:version/networking/service-mesh',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/networking/consul/',
    destination: '/nomad/docs/:version/integrations/consul',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/acl/consul/',
    destination: '/nomad/docs/:version/integrations/consul/acl',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/networking/consul/service-mesh/',
    destination: '/nomad/docs/:version/integrations/consul/service-mesh',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/scale/benchmarking/',
    destination: '/nomad/docs/:version/operations/benchmarking',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/authentication/jwt/',
    destination: '/nomad/docs/:version/concepts/acl/auth-methods/jwt',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/authentication/oidc/',
    destination: '/nomad/docs/:version/concepts/acl/auth-methods/oidc',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/vault/',
    destination: '/nomad/docs/:version/integrations/vault',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/vault/',
    destination: '/nomad/docs/:version/integrations/vault',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/vault/acl/',
    destination: '/nomad/docs/:version/integrations/vault/acl',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/workload-identity/aws-oidc-provider/',
    destination: '/nomad/docs/:version/operations/aws-oidc-provider',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-declare/task-driver/',
    destination: '/nomad/docs/:version/drivers',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/reference/hcl2/',
    destination: '/nomad/docs/:version/job-specification/hcl2',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/reference/metrics/',
    destination: '/nomad/docs/:version/operations/metrics-reference',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/reference/runtime-environment-settings/',
    destination: '/nomad/docs/:version/runtime/environment',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/reference/runtime-variable-interpolation/',
    destination: '/nomad/docs/:version/runtime/interpolation',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/reference/sentinel-policy/',
    destination: '/nomad/docs/:version/enterprise/sentinel',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/use-cases/',
    destination: '/nomad/docs/:version/who-uses-nomad',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/reference/runtime-environment-variables/',
    destination: '/nomad/docs/:version/runtime/',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/reference/runtime-environment-settings/',
    destination: '/nomad/docs/:version/runtime',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/acl/',
    destination: '/nomad/tutorials/archive/access-control',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/acl/bootstrap/',
    destination: '/nomad/tutorials/archive/access-control-bootstrap',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/acl/policies/create-policy/',
    destination: '/nomad/tutorials/archive/access-control-create-policy',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/acl/policies/',
    destination: '/nomad/tutorials/archive/access-control-policies',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/acl/tokens/',
    destination: '/nomad/tutorials/archive/access-control-tokens',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/authentication/sso-auth0/',
    destination: '/nomad/tutorials/archive/sso-oidc-auth0',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/authentication/sso-pkce-jwt/',
    destination: '/nomad/tutorials/archive/sso-oidc-keycloak',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/acl/tokens/vault/',
    destination: '/nomad/tutorials/archive/vault-nomad-secrets',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-scheduling/',
    destination: '/nomad/tutorials/archive/advanced-scheduling',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-scheduling/affinity/',
    destination: '/nomad/tutorials/archive/affinity',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-scheduling/preemption/',
    destination: '/nomad/tutorials/archive/preemption',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-scheduling/spread/',
    destination: '/nomad/tutorials/archive/spread',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/govern/',
    destination: '/nomad/tutorials/archive/governance-and-policy',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/govern/resource-quotas/',
    destination: '/nomad/tutorials/archive/quotas',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/govern/sentinel/',
    destination: '/nomad/tutorials/archive/sentinel',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/monitor/event-stream/',
    destination: '/nomad/tutorials/archive/event-stream',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/workload-identity/vault-acl/',
    destination: '/nomad/tutorials/archive/vault-acl',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/acl/tokens/vault/',
    destination: '/nomad/tutorials/archive/vault-nomad-secrets',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-declare/failure/',
    destination: '/nomad/tutorials/archive/failures',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-declare/failure/check-restart/',
    destination: '/nomad/tutorials/archive/failures-check-restart',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-declare/failure/reschedule/',
    destination: '/nomad/tutorials/archive/failures-reschedule',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-declare/failure/restart/',
    destination: '/nomad/tutorials/archive/failures-restart',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-declare/nomad-actions/',
    destination: '/nomad/tutorials/archive/job-spec-actions',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-declare/strategy/blue-green-canary/',
    destination: '/nomad/tutorials/archive/job-blue-green-and-canary-deployments',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-declare/strategy/rolling/',
    destination: '/nomad/tutorials/archive/job-rolling-update',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-declare/exit-signals/',
    destination: '/nomad/tutorials/archive/job-update-handle-signals',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-declare/strategy/',
    destination: '/nomad/tutorials/archive/job-update-strategies',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/manage/autopilot/',
    destination: '/nomad/tutorials/archive/autopilot',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/deploy/clusters/connect-nodes/',
    destination: '/nomad/tutorials/archive/clustering',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/deploy/clusters/federate-regions/',
    destination: '/nomad/tutorials/archive/federation',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-declare/multiregion/',
    destination: '/nomad/tutorials/archive/multiregion-deployments',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/govern/namespaces/',
    destination: '/nomad/tutorials/archive/namespaces',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/manage/migrate-workloads/',
    destination: '/nomad/tutorials/archive/node-drain',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/manage/outage-recovery/',
    destination: '/nomad/tutorials/archive/outage-recovery',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/deploy/clusters/reverse-proxy-ui/',
    destination: '/nomad/tutorials/archive/reverse-proxy-ui',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-declare/',
    destination: '/nomad/tutorials/archive/jobs',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-run/logs/',
    destination: '/nomad/tutorials/archive/jobs-accessing-logs',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-declare/configure-tasks/',
    destination: '/nomad/tutorials/archive/jobs-configuring',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-run/inspect/',
    destination: '/nomad/tutorials/archive/jobs-inspect',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-declare/create-job/',
    destination: '/nomad/tutorials/archive/jobs-submit',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-run/utilization-metrics/',
    destination: '/nomad/tutorials/archive/jobs-utilization',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-run/versions/',
    destination: '/nomad/tutorials/archive/jobs-version',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)s/nomad-pack/advanced-usage/',
    destination: '/nomad/tutorials/archive/nomad-pack-detailed-usage',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)s/nomad-pack/',
    destination: '/nomad/tutorials/archive/nomad-pack-intro',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)s/nomad-pack/create-packs/',
    destination: '/nomad/tutorials/archive/nomad-pack-writing-packs',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)archiving b/c LXC plugin removed a while ago/',
    destination: '/nomad/tutorials/archive/plugin-lxc',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/authentication/sso-vault/',
    destination: '/nomad/tutorials/archive/sso-oidc-vault',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/stateful-workloads/static-host-volumes/',
    destination: '/nomad/tutorials/archive/exec-users-host-volumes',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/stateful-workloads/',
    destination: '/nomad/tutorials/archive/stateful-workloads',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/stateful-workloads/csi-volumes/',
    destination: '/nomad/tutorials/archive/stateful-workloads-csi-volumes',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/stateful-workloads/dynamic-host-volumes/',
    destination: '/nomad/tutorials/archive/stateful-workloads-dynamic-host-volumes',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/stateful-workloads/static-host-volumes/',
    destination: '/nomad/tutorials/archive/stateful-workloads-host-volumes',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-declare/task-dependencies/',
    destination: '/nomad/tutorials/archive/task-dependencies-interjob',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/manage/format-cli-output/',
    destination: '/nomad/tutorials/archive/format-output-with-templates',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/reference/go-template-syntax/',
    destination: '/nomad/tutorials/archive/go-template-syntax',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/traffic/',
    destination: '/nomad/tutorials/archive/security-concepts',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/traffic/tls/',
    destination: '/nomad/tutorials/archive/security-enable-tls',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/secure/traffic/gossip-encryption/',
    destination: '/nomad/tutorials/archive/security-gossip-encryption',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/job-declare/nomad-variables/',
    destination: '/nomad/tutorials/archive/variables-tasks',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/monitor/cluster-topology/',
    destination: '/nomad/tutorials/archive/topology-visualization',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/monitor/inspect-cluster/',
    destination: '/nomad/tutorials/archive/web-ui-cluster-info',
    permanent: true
  },
  {
    source: '/nomad/docs/:version(v1\.(?:8|9)\.x)/monitor/inspect-workloads/',
    destination: '/nomad/tutorials/archive/web-ui-workload-info',
    permanent: true
  },
]
