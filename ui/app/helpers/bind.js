/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';
import { assert } from '@ember/debug';

/**
 * bind
 *
 * Usage: {{bind this.function}}
 *
 * Returns a version of a function bound to the template target (e.g., component or controller)
 */
export function bind([func, target]) {
  assert(
    'A function is required as the first argument',
    typeof func === 'function'
  );
  assert('A context is required as the second argument', target);
  return func.bind(target);
}

export default helper(bind);
