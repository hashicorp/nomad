import {
  attribute,
  create,
  collection,
  clickable,
  fillable,
  is,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';

import facet from 'nomad-ui/tests/pages/components/facet';
import pageSizeSelect from 'nomad-ui/tests/pages/components/page-size-select';

export default create({
  pageSize: 25,

  visit: visitable('/jobs'),

  search: fillable('[data-test-jobs-search] input'),

  runJobButton: {
    scope: '[data-test-run-job]',
    isDisabled: is('[disabled]'),
  },

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

  nextPage: clickable('[data-test-pager="next"]'),
  prevPage: clickable('[data-test-pager="prev"]'),

  isEmpty: isPresent('[data-test-empty-jobs-list]'),
  emptyState: {
    headline: text('[data-test-empty-jobs-list-headline]'),
  },

  error: {
    isPresent: isPresent('[data-test-error]'),
    title: text('[data-test-error-title]'),
    message: text('[data-test-error-message]'),
    seekHelp: clickable('[data-test-error-message] a'),
    gotoJobs: clickable('[data-test-error-jobs-link]'),
    gotoClients: clickable('[data-test-error-clients-link]'),
  },

  namespaceSwitcher: {
    isPresent: isPresent('[data-test-namespace-switcher]'),
    open: clickable('[data-test-namespace-switcher] .ember-power-select-trigger'),
    options: collection('.ember-power-select-option', {
      testContainer: '#ember-testing',
      resetScope: true,
      label: text(),
    }),
  },

  pageSizeSelect: pageSizeSelect(),

  facets: {
    type: facet('[data-test-type-facet]'),
    status: facet('[data-test-status-facet]'),
    datacenter: facet('[data-test-datacenter-facet]'),
    prefix: facet('[data-test-prefix-facet]'),
  },
});
