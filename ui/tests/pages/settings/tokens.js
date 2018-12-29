import {
  create,
  collection,
  clickable,
  fillable,
  text,
  isVisible,
  visitable,
} from 'ember-cli-page-object';

export default create({
  visit: visitable('/settings/tokens'),

  secret: fillable('[data-test-token-secret]'),
  submit: clickable('[data-test-token-submit]'),

  errorMessage: isVisible('[data-test-token-error]'),
  successMessage: isVisible('[data-test-token-success]'),
  managementMessage: isVisible('[data-test-token-management-message]'),

  policies: collection('[data-test-token-policy]', {
    name: text('[data-test-policy-name]'),
    description: text('[data-test-policy-description]'),
    rules: text('[data-test-policy-rules]', { normalize: false }),
  }),
});
