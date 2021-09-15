'use strict';

module.exports = {
  root: true,
  globals: {
    server: true,
  },
  env: {
    browser: true,
    es6: true,
  },
  extends: [
    'eslint:recommended',
    'plugin:ember/recommended',
  ],
  parser: 'babel-eslint',
  parserOptions: {
    ecmaVersion: 2018,
    sourceType: 'module',
    ecmaFeatures: {
      legacyDecorators: true,
    },
  },
  plugins: [
    'ember'
  ],
  rules: {
    indent: ['error', 2, { SwitchCase: 1 }],
    'linebreak-style': ['error', 'unix'],
    quotes: ['error', 'single', 'avoid-escape'],
    semi: ['error', 'always'],
    'no-constant-condition': [
      'error',
      {
        checkLoops: false,
      },
    ],
    'ember/classic-decorator-hooks': 'error',
    'ember/classic-decorator-no-classic-methods': 'error',
    'ember/no-get': 'off',
    'ember/no-mixins': 'off',
  },
  overrides: [
    // node files
    {
      files: [
        '.eslintrc.js',
        '.template-lintrc.js',
        'ember-cli-build.js',
        'testem.js',
        'blueprints/*/index.js',
        'config/**/*.js',
        'server/**/*.js',
        'lib/*/index.js',
      ],
      parserOptions: {
        sourceType: 'script',
      },
      env: {
        browser: false,
        node: true,
      },
      plugins: ['node'],
      rules: {
        'node/no-unpublished-require': 'off'
      },
    },
    {
      files: [
        'stories/**/*.js'
      ],
      parserOptions: {
        sourceType: 'module',
      },
      env: {
        browser: false,
        node: true,
      },
      plugins: ['node'],
    },
  ],
};
