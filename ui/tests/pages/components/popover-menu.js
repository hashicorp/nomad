/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  clickable,
  focusable,
  isPresent,
  text,
  triggerable,
} from 'ember-cli-page-object';

const ARROW_DOWN = 40;
const ESC = 27;
const TAB = 9;

export default (scope) => ({
  scope,

  isPresent: isPresent(),
  label: text('[data-test-popover-trigger]'),
  labelHasIcon: isPresent('[data-test-popover-trigger] svg.icon'),

  toggle: clickable('[data-test-popover-trigger]'),
  focus: focusable('[data-test-popover-trigger]'),
  downArrow: triggerable('keyup', '[data-test-popover-trigger]', {
    eventProperties: { keyCode: ARROW_DOWN },
  }),
  focusNext: triggerable('keyup', '[data-test-popover-trigger]', {
    eventProperties: { keyCode: TAB },
  }),
  esc: triggerable('keydown', '[data-test-popover-trigger]', {
    eventProperties: { keyCode: ESC },
  }),

  menu: {
    scope: '[data-test-popover-menu]',
    testContainer: '#ember-testing',
    resetScope: true,
    isOpen: isPresent(),
  },
});
