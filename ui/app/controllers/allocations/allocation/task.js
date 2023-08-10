/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';

export default class AllocationsAllocationTaskController extends Controller {
  get task() {
    return this.model;
  }

  get breadcrumb() {
    return {
      title: 'Task',
      label: this.task.get('name'),
      args: [
        'allocations.allocation.task',
        this.task.get('allocation'),
        this.task,
      ],
    };
  }
}
