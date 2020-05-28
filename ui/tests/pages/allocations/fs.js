import {
  attribute,
  collection,
  clickable,
  create,
  hasClass,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';

export default create({
  visit: visitable('/allocations/:id/:name/fs'),
  visitPath: visitable('/allocations/:id/:name/fs/:path'),

  visitAllocation: visitable('/allocations/:id/fs'),
  visitAllocationPath: visitable('/allocations/:id/fs/:path'),

  fileViewer: {
    scope: '[data-test-file-viewer]',
  },

  breadcrumbsText: text('[data-test-fs-breadcrumbs]'),

  breadcrumbs: collection('[data-test-fs-breadcrumbs] li', {
    visit: clickable('a'),
    path: attribute('href', 'a'),
    isActive: hasClass('is-active'),
  }),

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

  directoryEntries: collection('[data-test-entry]', {
    name: text('[data-test-name]'),

    isFile: isPresent('.icon-is-file-outline'),
    isDirectory: isPresent('.icon-is-folder-outline'),

    size: text('[data-test-size]'),
    lastModified: text('[data-test-last-modified]'),

    visit: clickable('a'),
    path: attribute('href', 'a'),
  }),

  isEmptyDirectory: isPresent('[data-test-empty-directory]'),

  directoryEntryNames() {
    return this.directoryEntries.toArray().mapBy('name');
  },

  hasEmptyState: isPresent('[data-test-not-running]'),
  emptyState: {
    headline: text('[data-test-not-running-headline]'),
  },

  error: {
    title: text('[data-test-error-title]'),
  },
});
