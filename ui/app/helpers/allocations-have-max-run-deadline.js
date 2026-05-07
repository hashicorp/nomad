/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';

export function allocationsHaveMaxRunDeadline([allocations]) {
  const items = allocations?.toArray?.() || allocations || [];
  return items.some((allocation) => allocation?.maxRunDeadline);
}

export default Helper.helper(allocationsHaveMaxRunDeadline);
