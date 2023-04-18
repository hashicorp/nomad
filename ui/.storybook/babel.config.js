/* eslint-env node */

// Inject the named blocks polyfill into the template compiler (and then into the babel plugin)
const templateCompiler = require('ember-source/dist/ember-template-compiler');
const namedBlocksPolyfillPlugin = require('ember-named-blocks-polyfill/lib/named-blocks-polyfill-plugin');

templateCompiler.registerPlugin('ast', namedBlocksPolyfillPlugin);

module.exports = {
  presets: [
    [
      '@babel/preset-env',
      {
        shippedProposals: true,
        useBuiltIns: 'usage',
        corejs: '3',
        targets: [
          'last 1 Chrome versions',
          'last 1 Firefox versions',
          'last 1 Safari versions',
        ],
      },
    ],
  ],
  plugins: [
    [
      '@babel/plugin-proposal-decorators',
      {
        legacy: true,
      },
    ],
    ['@babel/plugin-proposal-class-properties', { loose: true }],
    '@babel/plugin-syntax-dynamic-import',
    [
      '@babel/plugin-proposal-object-rest-spread',
      { loose: true, useBuiltIns: true },
    ],
    'babel-plugin-macros',
    ['emotion', { sourceMap: true, autoLabel: true }],
    [
      'babel-plugin-htmlbars-inline-precompile',
      {
        precompile: templateCompiler.precompile,
        modules: {
          'ember-cli-htmlbars': 'hbs',
          'ember-cli-htmlbars-inline-precompile': 'default',
          'htmlbars-inline-precompile': 'default',
        },
      },
      // This is an arbitrary label to prevent a collision with the existing htmlbars inline precompile
      // plugin that comes in from the @storybook/ember defaults.
      // TODO: After upgrading to Storybook 6.1 this should move into the new emberOptions construct.
      'override',
    ],
  ],
};
