/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import {
  click,
  // fillIn,
  // triggerKeyEvent,
  // triggerEvent,
} from '@ember/test-helpers';

/**
 * @param {string} scope
 * @param {*} options
 */
export async function clickToggle(scope, options) {
  let selector = '.hds-dropdown-toggle-button';
  if (scope) {
    selector = `${scope} ${selector}`;
  }
  return click(selector, options);
}

/**
 * @param {string} scope
 * @param {string} option the name of the option to click
 * @param {*} options
 */
export async function clickOption(scope, option, options) {
  let selector = `.hds-dropdown-list-item label input[name="${option}"]`;
  if (scope) {
    selector = `${scope} ${selector}`;
  }
  return click(selector, options);
}
