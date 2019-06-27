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

  fileViewer: {
    scope: '[data-test-file-viewer]',
  },

  breadcrumbsText: text('[data-test-fs-breadcrumbs]'),

  breadcrumbs: collection('[data-test-fs-breadcrumbs] li', {
    visit: clickable('a'),
    isActive: hasClass('is-active'),
  }),

  directoryEntries: collection('[data-test-entry]', {
    name: text('[data-test-name]'),

    isFile: isPresent('[data-test-file-icon]'),
    isDirectory: isPresent('[data-test-directory-icon]'),
    isEmpty: isPresent('[data-test-empty-icon]'),

    size: text('[data-test-size]'),
    lastModified: text('[data-test-last-modified]'),

    visit: clickable('a'),
    path: attribute('href', 'a'),
  }),

  hasEmptyState: isPresent('[data-test-not-running]'),
  emptyState: {
    headline: text('[data-test-not-running-headline]'),
  },
});
