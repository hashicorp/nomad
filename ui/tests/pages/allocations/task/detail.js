import {
  attribute,
  create,
  collection,
  clickable,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';

export default create({
  visit: visitable('/allocations/:id/:name'),

  title: text('[data-test-title]'),
  state: text('[data-test-state]'),
  startedAt: text('[data-test-started-at]'),

  breadcrumbs: collection('[data-test-breadcrumb]', {
    id: attribute('data-test-breadcrumb'),
    text: text(),
    visit: clickable(),
  }),

  breadcrumbFor(id) {
    return this.breadcrumbs.toArray().find(crumb => crumb.id === id);
  },

  resourceCharts: collection('[data-test-primary-metric]', {
    name: text('[data-test-primary-metric-title]'),
    chartClass: attribute('class', '[data-test-percentage-chart] progress'),
  }),

  resourceEmptyMessage: text('[data-test-resource-error-headline]'),

  hasAddresses: isPresent('[data-test-task-addresses]'),
  addresses: collection('[data-test-task-address]', {
    name: text('[data-test-task-address-name]'),
    isDynamic: text('[data-test-task-address-is-dynamic]'),
    address: text('[data-test-task-address-address]'),
  }),

  events: collection('[data-test-task-event]', {
    time: text('[data-test-task-event-time]'),
    type: text('[data-test-task-event-type]'),
    message: text('[data-test-task-event-message]'),
  }),

  error: {
    isPresent: isPresent('[data-test-error]'),
    title: text('[data-test-error-title]'),
    message: text('[data-test-error-message]'),
    seekHelp: clickable('[data-test-error-message] a'),
  },
});
