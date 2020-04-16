import { create, clickable, collection, isPresent, text } from 'ember-cli-page-object';
import { findElementWithAssert } from 'ember-cli-page-object/extend';

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

      groups: collection('.ember-power-select-group', {
        testContainer: '.ember-power-select-options',
        resetScope: true,
        name: text('.ember-power-select-group-name'),

        options: collection('.ember-power-select-option', {
          label: text(),
          statusClass: statusClass('.color-swatch'),
        }),
      }),

      hasNoMatches: isPresent('.ember-power-select-option--no-matches-message', {
        testContainer: 'html',
        resetScope: true,
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
  },
});

function statusClass(selector, options = {}) {
  return {
    isDescriptor: true,

    get() {
      const element = findElementWithAssert(this, selector, options)[0];
      return Array.from(element.classList)[1];
    },
  };
}
