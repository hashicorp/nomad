/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { get } from '@ember/object';
import { helper } from '@ember/component/helper';

/**
 * Eq By
 *
 * Usage: {{eq-by "prop" obj1 obj2}}
 *
 * Returns true when obj1 and obj2 have the same value for property "prop"
 */
export function eqBy([prop, obj1, obj2]) {
  if (!prop || !obj1 || !obj2) return false;
  return get(obj1, prop) === get(obj2, prop);
}

export default helper(eqBy);
