/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model from '@ember-data/model';
import { attr } from '@ember-data/model';

export default class JobDispatch extends Model {
  @attr() index;
  @attr() jobCreateIndex;
  @attr() evalCreateIndex;
  @attr() evalID;
  @attr() dispatchedJobID;
}
