/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

'use strict';

let USE_MIRAGE = true;

if (process.env.USE_MIRAGE) {
  USE_MIRAGE = process.env.USE_MIRAGE == 'true';
}

let USE_WATCHER_WEBSOCKETS = true;

if (process.env.USE_WATCHER_WEBSOCKETS) {
  USE_WATCHER_WEBSOCKETS = process.env.USE_WATCHER_WEBSOCKETS == 'true';
}

// Usage of websockets is not allowed in FIPS-140 mode due to lack
// of SHA1. Check if FIPS mode has been enabled for the go build and,
// if so, disable websockets use for watchers.
if (process.env.GOFIPS140) {
  USE_WATCHER_WEBSOCKETS = false;
}

module.exports = function (environment) {
  const ENV = {
    modulePrefix: 'nomad-ui',
    environment,
    rootURL: '/ui/',
    locationType: 'history',
    EmberENV: {
      EXTEND_PROTOTYPES: false,
      FEATURES: {
        // Here you can enable experimental features on an ember canary build
        // e.g. EMBER_NATIVE_DECORATOR_SUPPORT: true
      },
    },
    emberFlightIcons: {
      lazyEmbed: true,
    },
    APP: {
      blockingQueries: true,
      mirageScenario: 'smallCluster',
      mirageWithNamespaces: true,
      mirageWithTokens: true,
      mirageWithRegions: true,
      watcherWebSockets: USE_WATCHER_WEBSOCKETS,
    },
  };

  if (environment === 'development') {
    // ENV.APP.LOG_RESOLVER = true;
    // ENV.APP.LOG_ACTIVE_GENERATION = true;
    // ENV.APP.LOG_TRANSITIONS = true;
    // ENV.APP.LOG_TRANSITIONS_INTERNAL = true;
    // ENV.APP.LOG_VIEW_LOOKUPS = true;

    ENV['ember-cli-mirage'] = {
      enabled: USE_MIRAGE,
      excludeFilesFromBuild: !USE_MIRAGE,
    };
  }

  if (environment === 'test') {
    // Testem prefers this...
    ENV.locationType = 'none';

    // keep test console output quieter
    ENV.APP.LOG_ACTIVE_GENERATION = false;
    ENV.APP.LOG_VIEW_LOOKUPS = false;

    ENV.APP.rootElement = '#ember-testing';
    ENV.APP.autoboot = false;

    // disable watcher sockets while testing
    ENV.APP.watcherWebSockets = false;
  }

  if (environment === 'production') {
    // here you can enable a production-specific feature
  }

  return ENV;
};
