/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Fragment from 'ember-data-model-fragments/fragment';
import { attr } from '@ember-data/model';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';

export default class TaskGroup extends Fragment {
  @fragmentOwner() taskGroup;

  @attr('boolean') enabled;
  @attr('number') max;
  @attr('number') min;

  @attr() policy;
}
