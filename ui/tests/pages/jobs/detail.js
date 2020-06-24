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

export default create({
  visit: visitable('/jobs/:id'),

  tabs: collection('[data-test-tab]', {
    id: attribute('data-test-tab'),
    visit: clickable('a'),
  }),

  tabFor(id) {
    return this.tabs.toArray().findBy('id', id);
  },

  stop: twoStepButton('[data-test-stop]'),
  start: twoStepButton('[data-test-start]'),

  execButton: {
    scope: '[data-test-exec-button]',
    isDisabled: property('disabled'),
    hasTooltip: hasClass('tooltip'),
    tooltipText: attribute('aria-label'),
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
