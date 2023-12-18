/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

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

import pageSizeSelect from 'nomad-ui/tests/pages/components/page-size-select';

const heliosFacet = (scope) => ({
  scope,
  toggle: clickable('button'),
  options: collection(
    '.hds-menu-primitive__content .hds-dropdown__content .hds-dropdown__list .hds-dropdown-list-item--variant-checkbox',
    {
      toggle: clickable('label'),
      count: text('label .hds-dropdown-list-item__count'),
      key: attribute(
        'data-test-dropdown-option',
        '[data-test-dropdown-option]'
      ),
    }
  ),
});

export default create({
  pageSize: 25,

  visit: visitable('/clients'),

  search: fillable('.search-box input'),

  sortOptions: collection('[data-test-sort-by]', {
    id: attribute('data-test-sort-by'),
    sort: clickable(),
  }),

  sortBy(id) {
    return this.sortOptions.toArray().findBy('id', id).sort();
  },

  nodes: collection('[data-test-client-node-row]', {
    id: text('[data-test-client-id]'),
    name: text('[data-test-client-name]'),

    compositeStatus: {
      scope: '[data-test-client-composite-status]',

      tooltip: attribute('aria-label', '.tooltip'),

      isInfo: hasClass('is-info'),
      isSuccess: hasClass('is-success'),
      isWarning: hasClass('is-warning'),
      isUnformatted: isHidden('.status-text'),
    },

    address: text('[data-test-client-address]'),
    nodePool: text('[data-test-client-node-pool]'),
    datacenter: text('[data-test-client-datacenter]'),
    version: text('[data-test-client-version]'),
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
    nodePools: heliosFacet('[data-test-node-pool-facet]'),
    class: heliosFacet('[data-test-class-facet]'),
    state: heliosFacet('[data-test-state-facet]'),
    datacenter: heliosFacet('[data-test-datacenter-facet]'),
    version: heliosFacet('[data-test-version-facet]'),
    volume: heliosFacet('[data-test-volume-facet]'),
  },
});
