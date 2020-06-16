import { create, collection, clickable, text, visitable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/servers/:name'),

  tags: collection('[data-test-server-tag]', {
    name: text('td', { at: 0 }),
    value: text('td', { at: 1 }),
  }),

  error: {
    title: text('[data-test-error-title]'),
    message: text('[data-test-error-message]'),
    seekHelp: clickable('[data-test-error-message] a'),
  },
});
