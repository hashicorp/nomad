/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Helper from '@ember/component/helper';
import { formatScheduledBytes } from 'nomad-ui/utils/units';

/**
 * Scheduled Bytes Formatter
 *
 * Usage: {{format-scheduled-bytes bytes start="KiB"}}
 *
 * Outputs the bytes reduced to the resolution the scheduler
 * and job spec operate at.
 */
function formatScheduledBytesHelper([bytes], { start }) {
  return formatScheduledBytes(bytes, start);
}

export default Helper.helper(formatScheduledBytesHelper);
