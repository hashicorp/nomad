/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Helper from '@ember/component/helper';

export function isObject([value]) {
  const isObject =
    !Array.isArray(value) && value !== null && typeof value === 'object';
  return isObject;
}

export default Helper.helper(isObject);
