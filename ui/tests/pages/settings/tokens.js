/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

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
  clear: clickable('[data-test-token-clear]'),

  errorMessage: isVisible('[data-test-token-error]'),
  successMessage: isVisible('[data-test-token-success]'),
  managementMessage: isVisible('[data-test-token-management-message]'),
  ssoErrorMessage: isVisible('[data-test-sso-error]'),
  clearSSOError: clickable('[data-test-sso-error-clear]'),

  policies: collection('[data-test-token-policy]', {
    name: text('[data-test-policy-name]'),
    description: text('[data-test-policy-description]'),
    rules: text('[data-test-policy-rules]', { normalize: false }),
  }),
});
