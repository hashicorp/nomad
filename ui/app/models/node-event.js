/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { alias } from '@ember/object/computed';
import Fragment from 'ember-data-model-fragments/fragment';
import { attr } from '@ember-data/model';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default class NodeEvent extends Fragment {
  @fragmentOwner() node;

  @attr('string') message;
  @attr('string') subsystem;
  @attr() details;
  @attr('date') time;

  @alias('details.driver') driver;
}
