import { create, isPresent, text, visitable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/allocations/:id/:name/fs'),

  hasFiles: isPresent('[data-test-file-explorer]'),

  hasEmptyState: isPresent('[data-test-not-running]'),
  emptyState: {
    headline: text('[data-test-not-running-headline]'),
  },
});
