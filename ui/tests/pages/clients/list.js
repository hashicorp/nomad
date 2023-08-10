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

import { multiFacet } from 'nomad-ui/tests/pages/components/facet';
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
    return this.sortOptions.toArray().findBy('id', id).sort();
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
    nodePools: multiFacet('[data-test-node-pool-facet]'),
    class: multiFacet('[data-test-class-facet]'),
    state: multiFacet('[data-test-state-facet]'),
    datacenter: multiFacet('[data-test-datacenter-facet]'),
    version: multiFacet('[data-test-version-facet]'),
    volume: multiFacet('[data-test-volume-facet]'),
  },
});
