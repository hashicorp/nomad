/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

/**
 * Percentage Calculator
 *
 * Usage: {{format-percentage count total="number" complement="number"}}
 *
 * Outputs the ratio as a percentage, where ratio is either
 * `count / total` or `count / (count + complement)`
 */
export function formatPercentage(params, options = {}) {
  const value = safeNumber(params[0]);
  const complement = options.complement;

  let ratio;
  let total = options.total;

  if (total != undefined) {
    total = safeNumber(total);
  } else if (complement != undefined) {
    total = value + safeNumber(complement);
  } else {
    // Ensures that ratio is between 0 and 1 when neither total or complement are defined
    total = value;
  }

  // Use zero instead of infinity when the divisor is zero
  ratio = total ? value / total : 0;

  return 0 < ratio && ratio < 0.01 ? '< 1%' : Math.round(ratio * 100) + '%';
}

// If a value is not a number, treat it as zero
function safeNumber(value) {
  return isNaN(value) ? 0 : +value;
}

export default helper(formatPercentage);
