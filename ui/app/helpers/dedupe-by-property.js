/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// Takes an array and a property name and returns a new array with all the duplicates removed.
import { helper } from '@ember/component/helper';

export default helper(function dedupeByProperty([arr], { prop }) {
  const seen = new Set();
  return arr.filter((item) => {
    const val = item[prop];
    if (seen.has(val)) {
      return false;
    } else {
      seen.add(val);
      return true;
    }
  });
});
