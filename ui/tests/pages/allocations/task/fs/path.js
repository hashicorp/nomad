import { collection, create, isPresent, text, visitable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/allocations/:id/:name/fs/:path'),

  tempTitle: text('h1.title'),

  entries: collection('[data-test-entry]', {
    name: text('[data-test-name]'),
    isFile: isPresent('[data-test-file-icon]'),
    isDirectory: isPresent('[data-test-directory-icon]'),

    size: text('[data-test-size]'),
    fileMode: text('[data-test-file-mode]'),
  }),
});
