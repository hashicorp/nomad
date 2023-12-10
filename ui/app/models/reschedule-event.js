/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Fragment from 'ember-data-model-fragments/fragment';
import { attr } from '@ember-data/model';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';
import shortUUIDProperty from '../utils/properties/short-uuid';

export default class RescheduleEvent extends Fragment {
  @fragmentOwner() allocation;

  @attr('string') previousAllocationId;
  @attr('string') previousNodeId;
  @attr('date') time;
  @attr('string') delay;

  @shortUUIDProperty('previousAllocationId') previousAllocationShortId;
  @shortUUIDProperty('previousNodeShortId') previousNodeShortId;
}
