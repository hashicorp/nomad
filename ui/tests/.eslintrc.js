module.exports = {
  globals: {
    server: true,
    selectChoose: true,
    selectSearch: true,
    removeMultipleOption: true,
    clearSelected: true,
    getCodeMirrorInstance: true,
  },
  env: {
    embertest: true,
  },
  overrides: {
    files: ['acceptance/**/*-test.js'],
    plugins: ['ember-a11y-testing'],
    rules: {
      'ember-a11y-testing/a11y-audit-called': 'error',
    },
    settings: {
      'ember-a11y-testing': {
        auditModule: {
          package: 'nomad-ui/tests/helpers/a11y-audit',
          exportName: 'default',
        },
      },
    },
  },
};
