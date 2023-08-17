/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import AdapterError from '@ember-data/adapter/error';

export const NO_LEADER = 'No cluster leader';

export default class NoLeaderError extends AdapterError {
  message = NO_LEADER;
}
