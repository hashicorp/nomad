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
  },
});
