import {
  attribute,
  clickable,
  create,
  collection,
  fillable,
  visitable,
} from 'ember-cli-page-object';

import allocations from 'nomad-ui/tests/pages/components/allocations';

export default create({
  visit: visitable('/jobs/:id/allocations'),

  pageSize: 25,

  search: fillable('[data-test-allocations-search] input'),

  ...allocations(),

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
});
