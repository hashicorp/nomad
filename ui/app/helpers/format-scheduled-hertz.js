/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { formatScheduledHertz } from 'nomad-ui/utils/units';

/**
 * Scheduled Hertz Formatter
 *
 * Usage: {{format-scheduled-hertz hertz}}
 *
 * Outputs the frequency reduced to the resolution the scheduler
 * and job spec operate at.
 */
function formatScheduledHertzHelper([hertz], { start }) {
  return formatScheduledHertz(hertz, start || 'MHz');
}

export default Helper.helper(formatScheduledHertzHelper);
