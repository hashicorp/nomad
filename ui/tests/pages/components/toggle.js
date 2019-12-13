import {
  attribute,
  property,
  clickable,
  focusable,
  hasClass,
  isPresent,
  text,
  triggerable,
} from 'ember-cli-page-object';

const SPACE = 32;

export default scope => ({
  scope,

  isPresent: isPresent(),
  isDisabled: attribute('disabled', '[data-test-input]'),
  isActive: property('checked', '[data-test-input]'),

  hasDisabledClass: hasClass('is-disabled', '[data-test-label]'),
  hasActiveClass: hasClass('is-active', '[data-test-label]'),

  label: text('[data-test-label]'),

  toggle: clickable('[data-test-input]'),

  focus: focusable('[data-test-input]'),
  spaceBar: triggerable('keypress', '[data-test-input]', { eventProperties: { keyCode: SPACE } }),
});
