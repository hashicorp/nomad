/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Helper from '@ember/component/helper';
import formatDuration from '../utils/format-duration';

function formatDurationHelper([duration], { units, longForm }) {
  return formatDuration(duration, units, longForm);
}

export default Helper.helper(formatDurationHelper);
