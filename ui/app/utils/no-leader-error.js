/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import AdapterError from '@ember-data/adapter/error';

export const NO_LEADER = 'No cluster leader';

export default class NoLeaderError extends AdapterError {
  message = NO_LEADER;
}
