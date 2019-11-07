import {
  attribute,
  create,
  collection,
  clickable,
  fillable,
  hasClass,
  isHidden,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';

import facet from 'nomad-ui/tests/pages/components/facet';

export default create({
  visit: visitable('/clients'),

  search: fillable('.search-box input'),

  nodes: collection('[data-test-client-node-row]', {
    id: text('[data-test-client-id]'),
    name: text('[data-test-client-name]'),

    state: {
      scope: '[data-test-client-state]',

      tooltip: attribute('aria-label', '.tooltip'),

      isInfo: hasClass('is-info', '.status-text'),
      isWarning: hasClass('is-warning', '.status-text'),
      isUnformatted: isHidden('.status-text'),
    },

    address: text('[data-test-client-address]'),
    datacenter: text('[data-test-client-datacenter]'),
    allocations: text('[data-test-client-allocations]'),

    clickRow: clickable(),
    clickName: clickable('[data-test-client-name] a'),
  }),

  hasPagination: isPresent('[data-test-pagination]'),

  isEmpty: isPresent('[data-test-empty-clients-list]'),
  empty: {
    headline: text('[data-test-empty-clients-list-headline]'),
  },

  error: {
    isPresent: isPresent('[data-test-error]'),
    title: text('[data-test-error-title]'),
    message: text('[data-test-error-message]'),
    seekHelp: clickable('[data-test-error-message] a'),
  },

  facets: {
    class: facet('[data-test-class-facet]'),
    state: facet('[data-test-state-facet]'),
    datacenter: facet('[data-test-datacenter-facet]'),
  },
});
