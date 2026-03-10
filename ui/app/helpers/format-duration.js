/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import formatDuration from '../utils/format-duration';

function formatDurationHelper([duration], { units, longForm }) {
  return formatDuration(duration, units, longForm);
}

export default Helper.helper(formatDurationHelper);
