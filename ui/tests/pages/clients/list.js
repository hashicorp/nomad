import {
  create,
  collection,
  clickable,
  fillable,
  isPresent,
  text,
  visitable,
} from 'ember-cli-page-object';

export default create({
  visit: visitable('/clients'),

  search: fillable('.search-box input'),

  nodes: collection('[data-test-client-node-row]', {
    id: text('[data-test-client-id]'),
    name: text('[data-test-client-name]'),
    status: text('[data-test-client-status]'),
    drain: text('[data-test-client-drain]'),
    eligibility: text('[data-test-client-eligibility]'),
    address: text('[data-test-client-address]'),
    datacenter: text('[data-test-client-datacenter]'),
    allocations: text('[data-test-client-allocations]'),

    clickRow: clickable(),
    clickName: clickable('[data-test-client-name] a'),
  }),

  hasPagination: isPresent('[data-test-pagination]'),

  isEmpty: isPresent('[data-test-empty-clients-list]'),
  empty: {
    headline: text('[data-test-empty-clients-list-headline]'),
  },

  error: {
    isPresent: isPresent('[data-test-error]'),
    title: text('[data-test-error-title]'),
    message: text('[data-test-error-message]'),
    seekHelp: clickable('[data-test-error-message] a'),
  },
});
