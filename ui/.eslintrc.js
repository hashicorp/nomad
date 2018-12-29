module.exports = {
  globals: {
    server: true,
  },
  env: {
    browser: true,
    es6: true,
  },
  extends: 'eslint:recommended',
  parserOptions: {
    ecmaVersion: 2017,
    sourceType: 'module',
  },
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
  },
};
