/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// This serves as a lit of routes in the UI that we change over time,
// but still want to respect users' bookmarks and habits.

/**
 * @typedef {Object} RouteRedirect
 * @property {string} from - The path to match against
 * @property {(string|function(string): string)} to - Either a static path or a function to compute the new path
 * @property {'startsWith'|'exact'|'pattern'} method - The matching strategy to use
 * @property {RegExp} [pattern] - Optional regex pattern if method is 'pattern'
 */
export default [
  {
    from: '/csi/volumes/',
    to: (path) => {
      const volumeName = path.split('/csi/volumes/')[1];
      return `/storage/volumes/${volumeName}`;
    },
    method: 'pattern',
    pattern: /^\/csi\/volumes\/(.+)$/,
  },
  {
    from: '/csi/volumes',
    to: '/storage/volumes',
    method: 'exact',
  },
  {
    from: '/csi',
    to: '/storage',
    method: 'startsWith',
  },
];
