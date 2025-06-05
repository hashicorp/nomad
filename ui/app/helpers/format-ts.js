/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import moment from 'moment';
import Helper from '@ember/component/helper';

export function formatTs([date], options = {}) {
  if (options.relative) {
    return moment(date).fromNow();
  }
  const format = options.format
    ? options.format
    : options.short
    ? 'MMM D'
    : options.timeOnly
    ? 'HH:mm:ss'
    : "MMM DD, 'YY HH:mm:ss ZZ";
  return moment(date).format(format);
}

export default Helper.helper(formatTs);
