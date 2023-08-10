/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  attribute,
  clickable,
  create,
  collection,
  fillable,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';
import { multiFacet } from 'nomad-ui/tests/pages/components/facet';

import clients from 'nomad-ui/tests/pages/components/clients';
import error from 'nomad-ui/tests/pages/components/error';

export default create({
  visit: visitable('/jobs/:id/clients'),
  pageSize: 25,

  hasSearchBox: isPresent('[data-test-clients-search]'),
  search: fillable('[data-test-clients-search] input'),

  ...clients(),

  isEmpty: isPresent('[data-test-empty-clients-list]'),
  emptyState: {
    headline: text('[data-test-empty-clients-list-headline]'),
  },

  sortOptions: collection('[data-test-sort-by]', {
    id: attribute('data-test-sort-by'),
    sort: clickable(),
  }),

  sortBy(id) {
    return this.sortOptions.toArray().findBy('id', id).sort();
  },

  facets: {
    jobStatus: multiFacet('[data-test-job-status-facet]'),
    datacenter: multiFacet('[data-test-datacenter-facet]'),
    clientClass: multiFacet('[data-test-class-facet]'),
  },

  error: error(),
});
