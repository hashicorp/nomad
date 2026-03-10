/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import moment from 'moment';
import Helper from '@ember/component/helper';

export function formatMonthTs([date], options = {}) {
  const format = options.short ? 'MMM D' : 'MMM DD HH:mm:ss ZZ';
  return moment(date).format(format);
}

export default Helper.helper(formatMonthTs);
