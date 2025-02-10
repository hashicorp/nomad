/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import { attr } from '@ember-data/model';
import { fragmentOwner } from 'ember-data-model-fragments/attributes';
import Fragment from 'ember-data-model-fragments/fragment';

export default class ActionModel extends Fragment {
  @attr('string') name;
  @attr('string') command;

  /**
   * @type {string[]}
   */
  @attr() args;

  /**
   * @type {import('../models/task').default}
   */
  @fragmentOwner() task;

  /**
   * The allocations that the action could be run on.
   * @type {import('../models/allocation').default[]}
   */
  get allocations() {
    return this.task.taskGroup.allocations.filter((a) => {
      return a.clientStatus === 'running';
    });
  }
}
