import {
  attribute,
  create,
  collection,
  clickable,
  fillable,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';

export default create({
  pageSize: 10,

  visit: visitable('/jobs'),

  search: fillable('[data-test-jobs-search] input'),

  runJob: clickable('[data-test-run-job]'),

  jobs: collection('[data-test-job-row]', {
    id: attribute('data-test-job-row'),
    name: text('[data-test-job-name]'),
    link: attribute('href', '[data-test-job-name] a'),
    status: text('[data-test-job-status]'),
    type: text('[data-test-job-type]'),
    priority: text('[data-test-job-priority]'),
    taskGroups: text('[data-test-job-task-groups]'),

    clickRow: clickable(),
    clickName: clickable('[data-test-job-name] a'),
  }),

  isEmpty: isPresent('[data-test-empty-jobs-list]'),
  emptyState: {
    headline: text('[data-test-empty-jobs-list-headline]'),
  },

  error: {
    isPresent: isPresent('[data-test-error]'),
    title: text('[data-test-error-title]'),
    message: text('[data-test-error-message]'),
    seekHelp: clickable('[data-test-error-message] a'),
  },

  namespaceSwitcher: {
    isPresent: isPresent('[data-test-namespace-switcher]'),
    open: clickable('[data-test-namespace-switcher] .ember-power-select-trigger'),
    options: collection('.ember-power-select-option', {
      label: text(),
    }),
  },
});
