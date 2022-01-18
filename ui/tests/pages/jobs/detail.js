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
import jobClientStatusBar from 'nomad-ui/tests/pages/components/job-client-status-bar';

export default create({
  visit: visitable('/jobs/:id'),

  jobName: text('[data-test-job-name]'),

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

  packTag: isPresent('[data-test-pack-tag]'),
  metaTable: isPresent('[data-test-meta]'),

  execButton: {
    scope: '[data-test-exec-button]',
    isDisabled: property('disabled'),
    hasTooltip: hasClass('tooltip'),
    tooltipText: attribute('aria-label'),
  },

  incrementButton: {
    scope: '[data-test-scale-controls-increment]',
    isDisabled: property('disabled'),
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

  packStats: collection('[data-test-pack-stat]', {
    id: attribute('data-test-pack-stat'),
    text: text(),
  }),

  packStatFor(id) {
    return this.packStats.toArray().findBy('id', id);
  },

  jobClientStatusSummary: {
    scope: '[data-test-job-client-summary]',
    statusBar: jobClientStatusBar('[data-test-job-client-status-bar]'),
    toggle: {
      scope: '[data-test-accordion-head] [data-test-accordion-toggle]',
      click: clickable(),
      isDisabled: attribute('disabled'),
      tooltip: attribute('aria-label'),
    },
  },
  childrenSummary: jobClientStatusBar('[data-test-job-summary] [data-test-children-status-bar]'),
  allocationsSummary: jobClientStatusBar(
    '[data-test-job-summary] [data-test-allocation-status-bar]'
  ),
  ...allocations(),

  viewAllAllocations: text('[data-test-view-all-allocations]'),

  jobsHeader: {
    scope: '[data-test-jobs-header]',
    hasSubmitTime: isPresent('[data-test-jobs-submit-time-header]'),
    hasNamespace: isPresent('[data-test-jobs-namespace-header]'),
  },

  jobs: collection('[data-test-job-row]', {
    id: attribute('data-test-job-row'),
    name: text('[data-test-job-name]'),
    namespace: text('[data-test-job-namespace]'),
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
