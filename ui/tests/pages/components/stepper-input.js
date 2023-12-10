/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  attribute,
  blurrable,
  clickable,
  fillable,
  focusable,
  isPresent,
  text,
  triggerable,
  value,
} from 'ember-cli-page-object';

export default (scope) => ({
  scope,

  label: text('[data-test-stepper-label]'),
  clickLabel: clickable('[data-test-stepper-label]'),

  input: {
    scope: '[data-test-stepper-input]',
    fill: fillable(),
    focus: focusable(),
    blur: blurrable(),
    value: value(),
    esc: triggerable('keyup', '', { eventProperties: { keyCode: 27 } }),
    isDisabled: attribute('disabled'),
  },

  decrement: {
    scope: '[data-test-stepper-decrement]',
    click: clickable(),
    isPresent: isPresent(),
    isDisabled: attribute('disabled'),
    classNames: attribute('class'),
  },

  increment: {
    scope: '[data-test-stepper-increment]',
    click: clickable(),
    isPresent: isPresent(),
    isDisabled: attribute('disabled'),
    classNames: attribute('class'),
  },
});
