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
    destination: '/nomad/docs/job-declare/task-driver/:path*',
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
    destination: '/nomad/docs/networking/ipv6-support',
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
  // removed section index pages that had no meaningful content
  {
    source: '/nomad/docs/operations',
    destination: '/nomad/docs',
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
]
