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

  allocations: collection('[data-test-allocation]', {
    id: attribute('data-test-allocation'),
    shortId: text('[data-test-short-id]'),
    createTime: text('[data-test-create-time]'),
    modifyTime: text('[data-test-modify-time]'),
    status: text('[data-test-client-status]'),
    jobVersion: text('[data-test-job-version]'),
    client: text('[data-test-client]'),
    cpu: text('[data-test-cpu]'),
    cpuTooltip: attribute('aria-label', '[data-test-cpu] .tooltip'),
    mem: text('[data-test-mem]'),
    memTooltip: attribute('aria-label', '[data-test-mem] .tooltip'),
    rescheduled: isPresent('[data-test-indicators] [data-test-icon="reschedule"]'),

    visit: clickable('[data-test-short-id] a'),
    visitClient: clickable('[data-test-client] a'),
  }),

  allocationFor(id) {
    return this.allocations.toArray().find(allocation => allocation.id === id);
  },

  isEmpty: isPresent('[data-test-empty-allocations-list]'),

  emptyState: {
    headline: text('[data-test-empty-allocations-list-headline]'),
  },
});
