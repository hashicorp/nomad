/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  attribute,
  clickable,
  hasClass,
  isPresent,
  text,
} from 'ember-cli-page-object';

export default (scope) => ({
  scope,

  isPresent: isPresent(),

  idle: clickable('[data-test-idle-button]'),
  confirm: clickable('[data-test-confirm-button]'),
  cancel: clickable('[data-test-cancel-button]'),

  isRunning: hasClass('is-loading', '[data-test-confirm-button]'),
  isDisabled: attribute('disabled', '[data-test-idle-button]'),

  cancelIsDisabled: attribute('disabled', '[data-test-cancel-button]'),
  confirmIsDisabled: attribute('disabled', '[data-test-confirm-button]'),

  idleText: text('[data-test-idle-button]'),
  cancelText: text('[data-test-cancel-button]'),
  confirmText: text('[data-test-confirm-button]'),
  confirmationMessage: text('[data-test-confirmation-message]'),
});
