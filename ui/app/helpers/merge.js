/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

function merge(positional) {
  return positional.reduce((accum, val) => {
    accum = { ...val, ...accum };
    return accum;
  }, {});
}

export default helper(merge);
