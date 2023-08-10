/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';
import { dasherize } from '@ember/string';
/**
 * CSS Class
 *
 * Usage: {{css-class updateType}}
 *
 * Outputs a css friendly class string from any human string.
 * Differs from dasherize by handling slashes.
 */
export function cssClass([updateType]) {
  return dasherize(updateType.replace(/\//g, '-'));
}

export default helper(cssClass);
