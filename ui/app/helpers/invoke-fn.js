/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { helper } from '@ember/component/helper';

function invokeFn([scope, fn]) {
  let args = arguments[0].slice(2);
  return fn.apply(scope, args);
}

export default helper(invokeFn);
