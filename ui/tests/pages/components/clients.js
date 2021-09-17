import { attribute, collection, clickable, text } from 'ember-cli-page-object';
import { singularize } from 'ember-inflector';

export default function(selector = '[data-test-client]', propKey = 'clients') {
  const lookupKey = `${singularize(propKey)}For`;
  // Remove the bracket notation
  const attr = selector.substring(1, selector.length - 1);
  return {
    [propKey]: collection(selector, {
      id: attribute(attr),
      shortId: text('[data-test-short-id]'),
      createTime: text('[data-test-create-time]'),
      createTooltip: attribute('aria-label', '[data-test-create-time] .tooltip'),
      modifyTime: text('[data-test-modify-time]'),
      status: text('[data-test-client-status]'),
      job: text('[data-test-job]'),
      client: text('[data-test-client]'),

      visit: clickable('[data-test-short-id] a'),
      visitRow: clickable(),
      visitJob: clickable('[data-test-job]'),
      visitClient: clickable('[data-test-client] a'),
    }),

    [lookupKey]: function(id) {
      return this[propKey].toArray().find(client => client.id === id);
    },
  };
}
