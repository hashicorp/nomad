/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  clickable,
  collection,
  create,
  fillable,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';

import error from 'nomad-ui/tests/pages/components/error';
import { singleFacet } from 'nomad-ui/tests/pages/components/facet';
import pageSizeSelect from 'nomad-ui/tests/pages/components/page-size-select';

export default create({
  pageSize: 25,

  visit: visitable('/csi/volumes'),

  search: fillable('[data-test-volumes-search] input'),

  volumes: collection('[data-test-volume-row]', {
    name: text('[data-test-volume-name]'),
    namespace: text('[data-test-volume-namespace]'),
    schedulable: text('[data-test-volume-schedulable]'),
    controllerHealth: text('[data-test-volume-controller-health]'),
    nodeHealth: text('[data-test-volume-node-health]'),
    provider: text('[data-test-volume-provider]'),
    allocations: text('[data-test-volume-allocations]'),

    hasNamespace: isPresent('[data-test-volume-namespace]'),
    clickRow: clickable(),
    clickName: clickable('[data-test-volume-name] a'),
  }),

  nextPage: clickable('[data-test-pager="next"]'),
  prevPage: clickable('[data-test-pager="prev"]'),

  isEmpty: isPresent('[data-test-empty-volumes-list]'),
  emptyState: {
    headline: text('[data-test-empty-volumes-list-headline]'),
  },

  error: error(),
  pageSizeSelect: pageSizeSelect(),

  facets: {
    namespace: singleFacet('[data-test-namespace-facet]'),
  },
});
