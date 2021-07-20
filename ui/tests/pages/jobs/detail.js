import {
  attribute,
  create,
  collection,
  clickable,
  hasClass,
  isPresent,
  property,
  text,
  visitable,
} from 'ember-cli-page-object';

import allocations from 'nomad-ui/tests/pages/components/allocations';
import twoStepButton from 'nomad-ui/tests/pages/components/two-step-button';
import recommendationAccordion from 'nomad-ui/tests/pages/components/recommendation-accordion';

export default create({
  visit: visitable('/jobs/:id'),

  tabs: collection('[data-test-tab]', {
    id: attribute('data-test-tab'),
    visit: clickable('a'),
  }),

  tabFor(id) {
    return this.tabs.toArray().findBy('id', id);
  },

  recommendations: collection('[data-test-recommendation-accordion]', recommendationAccordion),

  stop: twoStepButton('[data-test-stop]'),
  start: twoStepButton('[data-test-start]'),

  execButton: {
    scope: '[data-test-exec-button]',
    isDisabled: property('disabled'),
    hasTooltip: hasClass('tooltip'),
    tooltipText: attribute('aria-label'),
  },

  dispatchButton: {
    scope: '[data-test-dispatch-button]',
    isDisabled: property('disabled'),
  },

  stats: collection('[data-test-job-stat]', {
    id: attribute('data-test-job-stat'),
    text: text(),
  }),

  statFor(id) {
    return this.stats.toArray().findBy('id', id);
  },

  childrenSummary: isPresent('[data-test-job-summary] [data-test-children-status-bar]'),
  allocationsSummary: isPresent('[data-test-job-summary] [data-test-allocation-status-bar]'),

  ...allocations(),

  viewAllAllocations: text('[data-test-view-all-allocations]'),

  jobs: collection('[data-test-job-row]', {
    id: attribute('data-test-job-row'),
    name: text('[data-test-job-name]'),
    link: attribute('href', '[data-test-job-name] a'),
    submitTime: text('[data-test-job-submit-time]'),
    status: text('[data-test-job-status]'),
    type: text('[data-test-job-type]'),
    priority: text('[data-test-job-priority]'),
    taskGroups: text('[data-test-job-task-groups]'),

    clickRow: clickable(),
    clickName: clickable('[data-test-job-name] a'),
  }),

  error: {
    isPresent: isPresent('[data-test-error]'),
    title: text('[data-test-error-title]'),
    message: text('[data-test-error-message]'),
    seekHelp: clickable('[data-test-error-message] a'),
  },

  recentAllocationsEmptyState: {
    headline: text('[data-test-empty-recent-allocations-headline]'),
  },
});
