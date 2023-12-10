/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';

export function isObject([value]) {
  const isObject =
    !Array.isArray(value) && value !== null && typeof value === 'object';
  return isObject;
}

export default Helper.helper(isObject);
