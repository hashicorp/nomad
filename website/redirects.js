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
  // Rename and re-arrange Autoscaling Internals section
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
    destination: '/nomad/tools/autoscaling/concepts/policy-eval/node-selector-strategy',
    permanent: true,
  },
]
