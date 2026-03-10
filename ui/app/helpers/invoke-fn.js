/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

function invokeFn([scope, fn]) {
  let args = arguments[0].slice(2);
  return fn.apply(scope, args);
}

export default helper(invokeFn);
