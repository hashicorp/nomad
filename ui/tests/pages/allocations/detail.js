import {
  attribute,
  clickable,
  create,
  collection,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';

export default create({
  visit: visitable('/allocations/:id'),

  title: text('[data-test-title]'),

  details: {
    scope: '[data-test-allocation-details]',

    job: text('[data-test-job-link]'),
    visitJob: clickable('[data-test-job-link]'),

    client: text('[data-test-client-link]'),
    visitClient: clickable('[data-test-client-link]'),
  },

  resourceCharts: collection('[data-test-primary-metric]', {
    name: text('[data-test-primary-metric-title]'),
    chartClass: attribute('class', '[data-test-percentage-chart] progress'),
  }),

  resourceEmptyMessage: text('[data-test-resource-error-headline]'),

  tasks: collection('[data-test-task-row]', {
    name: text('[data-test-name]'),
    state: text('[data-test-state]'),
    message: text('[data-test-message]'),
    time: text('[data-test-time]'),
    ports: text('[data-test-ports]'),

    hasUnhealthyDriver: isPresent('[data-test-icon="unhealthy-driver"]'),

    clickLink: clickable('[data-test-name] a'),
    clickRow: clickable('[data-test-name]'),
  }),

  firstUnhealthyTask() {
    return this.tasks.toArray().findBy('hasUnhealthyDriver');
  },

  hasRescheduleEvents: isPresent('[data-test-reschedule-events]'),

  isEmpty: isPresent('[data-test-empty-tasks-list]'),

  error: {
    isShown: isPresent('[data-test-error]'),
    title: text('[data-test-error-title]'),
    message: text('[data-test-error-message]'),
    seekHelp: clickable('[data-test-error-message] a'),
  },
});
