/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { formatBytes } from 'nomad-ui/utils/units';

/**
 * Bytes Formatter
 *
 * Usage: {{format-bytes bytes start="KiB"}}
 *
 * Outputs the bytes reduced to the largest supported unit size for which
 * bytes is larger than one.
 */
function formatBytesHelper([bytes], { start }) {
  return formatBytes(bytes, start);
}

export default Helper.helper(formatBytesHelper);
