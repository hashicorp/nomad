import { create, clickable, collection, isPresent, text } from 'ember-cli-page-object';

export default create({
  navbar: {
    scope: '[data-test-global-header]',

    regionSwitcher: {
      scope: '[data-test-region-switcher]',
      isPresent: isPresent(),
      open: clickable('.ember-power-select-trigger'),
      options: collection('.ember-power-select-option', {
        label: text(),
      }),
    },

    search: {
      scope: '[data-test-search]',

      click: clickable('.ember-power-select-trigger'),

      options: collection('.ember-power-select-option', {
        testContainer: '.ember-power-select-options',
        resetScope: true,
        label: text(),
      }),

      field: {
        scope: '.ember-power-select-dropdown--active',
        testContainer: 'html',
        resetScope: true,
      },
    },
  },

  gutter: {
    scope: '[data-test-gutter-menu]',
    namespaceSwitcher: {
      scope: '[data-test-namespace-switcher]',
      isPresent: isPresent(),
      open: clickable('.ember-power-select-trigger'),
      options: collection('.ember-power-select-option', {
        label: text(),
      }),
    },
    visitJobs: clickable('[data-test-gutter-link="jobs"]'),
    visitClients: clickable('[data-test-gutter-link="clients"]'),
    visitServers: clickable('[data-test-gutter-link="servers"]'),
    visitStorage: clickable('[data-test-gutter-link="storage"]'),
  },
});
