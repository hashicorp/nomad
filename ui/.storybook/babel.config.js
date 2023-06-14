/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

/* eslint-env node */

const templateCompiler = require('ember-source/dist/ember-template-compiler');

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
    ],
  ],
};
