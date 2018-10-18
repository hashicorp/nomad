/* eslint-env node */

module.exports = function(environment) {
  var ENV = {
    modulePrefix: 'nomad-ui',
    environment: environment,
    rootURL: '/ui/',
    locationType: 'auto',
    EmberENV: {
      FEATURES: {
        // Here you can enable experimental features on an ember canary build
        // e.g. 'with-controller': true
        'ember-routing-router-service': true,
      },
      EXTEND_PROTOTYPES: {
        // Prevent Ember Data from overriding Date.parse.
        Date: false,
      },
    },

    APP: {
      blockingQueries: true,
      mirageScenario: 'smallCluster',
      mirageWithNamespaces: false,
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
      // enabled: false,
    };
  }

  if (environment === 'test') {
    // Testem prefers this...
    ENV.locationType = 'none';

    // keep test console output quieter
    ENV.APP.LOG_ACTIVE_GENERATION = false;
    ENV.APP.LOG_VIEW_LOOKUPS = false;

    ENV.APP.rootElement = '#ember-testing';

    ENV.browserify = {
      tests: true,
    };

    ENV['ember-cli-mirage'] = {
      trackRequests: true,
    };
  }

  if (environment === 'production') {
  }

  return ENV;
};
