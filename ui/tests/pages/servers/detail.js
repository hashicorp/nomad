import { create, collection, clickable, hasClass, text, visitable } from 'ember-cli-page-object';
import { getter } from 'ember-cli-page-object/macros';

export default create({
  visit: visitable('/servers/:name'),

  servers: collection('[data-test-server-agent-row]', {
    name: text('[data-test-server-name]'),
    isActive: hasClass('is-active'),
  }),

  tags: collection('[data-test-server-tag]', {
    name: text('td', { at: 0 }),
    value: text('td', { at: 1 }),
  }),

  activeServer: getter(function() {
    return this.servers.toArray().find(server => server.isActive);
  }),

  error: {
    title: text('[data-test-error-title]'),
    message: text('[data-test-error-message]'),
    seekHelp: clickable('[data-test-error-message] a'),
  },
});
