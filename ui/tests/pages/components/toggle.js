/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  attribute,
  property,
  clickable,
  hasClass,
  isPresent,
  text,
} from 'ember-cli-page-object';

export default (scope) => ({
  scope,

  isPresent: isPresent(),
  isDisabled: attribute('disabled', '[data-test-input]'),
  isActive: property('checked', '[data-test-input]'),

  hasDisabledClass: hasClass('is-disabled', '[data-test-label]'),
  hasActiveClass: hasClass('is-active', '[data-test-label]'),

  label: text('[data-test-label]'),
  title: attribute('title'),

  toggle: clickable('[data-test-input]'),
});
