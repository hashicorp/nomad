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
    source: '/nomad/s/port-plan-failure',
    destination:
      'https://developer.hashicorp.com/nomad/docs/operations/monitoring-nomad#progress',
    permanent: false,
  },
  {
    source: '/nomad/s/envoy-bootstrap-error',
    destination:
      'https://developer.hashicorp.com/nomad/docs/integrations/consul/service-mesh#troubleshooting',
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
    destination: '/nomad/docs/integrations/consul',
    permanent: true,
  },
  {
    source: '/nomad/docs/integrations/consul-connect',
    destination: '/nomad/docs/integrations/consul/service-mesh',
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
    source: '/nomad/drivers/community/firecreacker-task-driver',
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
]
