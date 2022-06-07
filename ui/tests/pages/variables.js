import {
  create,
  visitable,
  clickable,
  collection,
} from 'ember-cli-page-object';

export default create({
  visit: visitable('/variables'),

  folders: collection('[data-test-folder-row]', {
    click: clickable(),
    clickLink: clickable('td a'),
  }),

  // folderLink: {
  //   scope: '[data-test-folder-row]',
  //   click: clickable(),
  // },

  // fileLink: {
  //   scope: '[data-test-file-row]',
  //   click: clickable(),
  // },
});
