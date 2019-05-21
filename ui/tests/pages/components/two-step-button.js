import { attribute, clickable, hasClass, isPresent } from 'ember-cli-page-object';

export default scope => ({
  scope,

  isPresent: isPresent(),

  idle: clickable('[data-test-idle-button]'),
  confirm: clickable('[data-test-confirm-button]'),
  cancel: clickable('[data-test-cancel-button]'),

  isRunning: hasClass('is-loading', '[data-test-confirm-button]'),
  isDisabled: attribute('disabled', '[data-test-idle-button]'),
});
