/* eslint-env node */
const EmberApp = require('ember-cli/lib/broccoli/ember-app');

const environment = EmberApp.env();
const isProd = environment === 'production';

module.exports = function(defaults) {
  var app = new EmberApp(defaults, {
    addons: {
      blacklist: isProd ? ['ember-freestyle'] : [],
    },
    svg: {
      paths: ['public/images/icons'],
    },
    codemirror: {
      modes: ['javascript'],
    },
    funnel: {
      enabled: isProd,
      exclude: [
        `${defaults.project.pkg.name}/components/freestyle/**/*`,
        `${defaults.project.pkg.name}/templates/components/freestyle/**/*`,
      ],
    },
    babel: {
      plugins: ['transform-object-rest-spread'],
    },
  });

  // Use `app.import` to add additional libraries to the generated
  // output files.
  //
  // If you need to use different assets in different
  // environments, specify an object as the first parameter. That
  // object's keys should be the environment name and the values
  // should be the asset to use in that environment.
  //
  // If the library that you are including contains AMD or ES6
  // modules that you would like to import into your application
  // please specify an object with the list of modules as keys
  // along with the exports of each module as its value.

  return app.toTree();
};
