/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  clickable,
  create,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';

import allocations from 'nomad-ui/tests/pages/components/allocations';
import { multiFacet } from 'nomad-ui/tests/pages/components/facet';
import pageSizeSelect from 'nomad-ui/tests/pages/components/page-size-select';

export default create({
  pageSize: 25,

  visit: visitable('/csi/plugins/:id/allocations'),

  nextPage: clickable('[data-test-pager="next"]'),
  prevPage: clickable('[data-test-pager="prev"]'),

  isEmpty: isPresent('[data-test-empty-jobs-list]'),
  emptyState: {
    headline: text('[data-test-empty-jobs-list-headline]'),
  },

  ...allocations('[data-test-allocation]', 'allocations'),

  pageSizeSelect: pageSizeSelect(),

  facets: {
    health: multiFacet('[data-test-health-facet]'),
    type: multiFacet('[data-test-type-facet]'),
  },
});
