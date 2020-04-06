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
import pageSizeSelect from 'nomad-ui/tests/pages/components/page-size-select';

export default create({
  pageSize: 25,

  visit: visitable('/clients'),

  search: fillable('.search-box input'),

  sortOptions: collection('[data-test-sort-by]', {
    id: attribute('data-test-sort-by'),
    sort: clickable(),
  }),

  sortBy(id) {
    return this.sortOptions
      .toArray()
      .findBy('id', id)
      .sort();
  },

  nodes: collection('[data-test-client-node-row]', {
    id: text('[data-test-client-id]'),
    name: text('[data-test-client-name]'),

    compositeStatus: {
      scope: '[data-test-client-composite-status]',

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

  pageSizeSelect: pageSizeSelect(),

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
    volume: facet('[data-test-volume-facet]'),
  },
});
