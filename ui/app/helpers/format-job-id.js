/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';

export function formatJobId([jobId]) {
  if (!jobId) return;
  return JSON.parse(jobId).join('@');
}

export default Helper.helper(formatJobId);
