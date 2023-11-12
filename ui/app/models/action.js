/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { attr } from '@ember-data/model';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';
import Fragment from 'ember-data-model-fragments/fragment';

export default class ActionModel extends Fragment {
  @attr('string') name;
  @attr('string') command;
  @attr() args;
  @fragmentOwner() task;

  get allocations() {
    return this.task.taskGroup.allocations.filter((a) => {
      return a.clientStatus === 'running';
    });
  }
}
