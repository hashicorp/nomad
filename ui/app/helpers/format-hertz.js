/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { formatHertz } from 'nomad-ui/utils/units';

/**
 * Hertz Formatter
 *
 * Usage: {{format-hertz hertz}}
 *
 * Outputs the frequency reduced to the largest supported unit size for which
 * the resulting number is larger than one.
 */
function formatHertzHelper([hertz], { start }) {
  return formatHertz(hertz, start || 'MHz');
}

export default Helper.helper(formatHertzHelper);
