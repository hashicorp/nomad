/* eslint-env node */

let USE_MIRAGE = true;

if (process.env.USE_MIRAGE) {
  USE_MIRAGE = process.env.USE_MIRAGE == 'true';
}

module.exports = function(environment) {
  var ENV = {
    modulePrefix: 'nomad-ui',
    environment: environment,
    rootURL: '/ui/',
    locationType: 'auto',
    EmberENV: {
      FEATURES: {
        // Here you can enable experimental features on an ember canary build
        // e.g. EMBER_NATIVE_DECORATOR_SUPPORT: true
      },
      EXTEND_PROTOTYPES: {
        // Prevent Ember Data from overriding Date.parse.
        Date: false,
      },
    },

    APP: {
      blockingQueries: true,
      mirageScenario: 'smallCluster',
      mirageWithNamespaces: true,
      mirageWithTokens: true,
      mirageWithRegions: true,
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

    ENV.browserify = {
      tests: true,
    };

    ENV['ember-cli-mirage'] = {
      trackRequests: true,
    };
  }

  // if (environment === 'production') {
  // }

  return ENV;
};
