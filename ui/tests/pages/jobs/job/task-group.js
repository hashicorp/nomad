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

import allocations from 'nomad-ui/tests/pages/components/allocations';
import error from 'nomad-ui/tests/pages/components/error';

export default create({
  pageSize: 10,

  visit: visitable('/jobs/:id/:name'),

  search: fillable('.search-box input'),

  tasksCount: text('[data-test-task-group-tasks]'),
  cpu: text('[data-test-task-group-cpu]'),
  mem: text('[data-test-task-group-mem]'),
  disk: text('[data-test-task-group-disk]'),

  breadcrumbs: collection('[data-test-breadcrumb]', {
    id: attribute('data-test-breadcrumb'),
    text: text(),
    visit: clickable(),
  }),

  breadcrumbFor(id) {
    return this.breadcrumbs.toArray().find(crumb => crumb.id === id);
  },

  ...allocations(),

  isEmpty: isPresent('[data-test-empty-allocations-list]'),

  error: error(),

  emptyState: {
    headline: text('[data-test-empty-allocations-list-headline]'),
  },
});
