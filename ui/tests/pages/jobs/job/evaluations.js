/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  attribute,
  clickable,
  create,
  collection,
  text,
  visitable,
} from 'ember-cli-page-object';

import error from 'nomad-ui/tests/pages/components/error';

export default create({
  visit: visitable('/jobs/:id/evaluations'),

  evaluations: collection('[data-test-evaluation]', {
    id: text('[data-test-id]'),
  }),

  sortOptions: collection('[data-test-sort-by]', {
    id: attribute('data-test-sort-by'),
    sort: clickable(),
  }),

  sortBy(id) {
    return this.sortOptions.toArray().findBy('id', id).sort();
  },

  error: error(),
});
