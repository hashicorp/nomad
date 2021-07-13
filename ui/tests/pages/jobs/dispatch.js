import { create, property, visitable } from 'ember-cli-page-object';

export default create({
  visit: visitable('/jobs/:id/dispatch'),

  dispatchButton: {
    scope: '[data-test-dispatch-button]',
    isDisabled: property('disabled'),
  },
});
