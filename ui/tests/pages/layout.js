import {
  attribute,
  create,
  clickable,
  collection,
  hasClass,
  isHidden,
  isPresent,
  text,
} from 'ember-cli-page-object';

export default create({
  navbar: {
    scope: '[data-test-global-header]',

    regionSwitcher: {
      scope: '[data-test-region-switcher-parent]',
      isPresent: isPresent(),
      open: clickable('.ember-power-select-trigger'),
      options: collection('.ember-power-select-option', {
        label: text(),
      }),
    },

    search: {
      scope: '[data-test-search-parent]',

      click: clickable('.ember-power-select-trigger'),

      groups: collection('.ember-power-select-group', {
        testContainer: '.ember-power-select-options',
        resetScope: true,
        name: text('.ember-power-select-group-name'),

        options: collection('.ember-power-select-option'),
      }),

      noOptionsShown: isHidden('.ember-power-select-options', {
        testContainer: '.ember-basic-dropdown-content',
        resetScope: true,
      }),

      field: {
        scope: '.ember-power-select-search input',
        testContainer: 'html',
        resetScope: true,
      },
    },
  },

  gutter: {
    scope: '[data-test-gutter-menu]',
    visitJobs: clickable('[data-test-gutter-link="jobs"]'),

    optimize: {
      scope: '[data-test-gutter-link="optimize"]',
    },

    visitClients: clickable('[data-test-gutter-link="clients"]'),
    visitServers: clickable('[data-test-gutter-link="servers"]'),
    visitStorage: clickable('[data-test-gutter-link="storage"]'),
  },

  breadcrumbs: collection('[data-test-breadcrumb]', {
    id: attribute('data-test-breadcrumb'),
    text: text(),
    visit: clickable(),
  }),

  breadcrumbFor(id) {
    return this.breadcrumbs.toArray().find(crumb => crumb.id === id);
  },

  error: {
    isPresent: isPresent('[data-test-error]'),
    title: text('[data-test-error-title]'),
    message: text('[data-test-error-message]'),
  },

  inlineError: {
    isShown: isPresent('[data-test-inline-error]'),
    title: text('[data-test-inline-error-title]'),
    message: text('[data-test-inline-error-body]'),
    dismiss: clickable('[data-test-inline-error-close]'),

    isDanger: hasClass('is-danger', '[data-test-inline-error]'),
    isWarning: hasClass('is-warning', '[data-test-inline-error]'),
  },
});
