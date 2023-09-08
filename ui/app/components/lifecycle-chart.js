/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { computed } from '@ember/object';
import { sort } from '@ember/object/computed';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('')
export default class LifecycleChart extends Component {
  tasks = null;
  taskStates = null;

  @computed('tasks.@each.lifecycle', 'taskStates.@each.state')
  get lifecyclePhases() {
    const tasksOrStates = this.taskStates || this.tasks;
    const lifecycles = {
      'prestart-ephemerals': [],
      'prestart-sidecars': [],
      'poststart-ephemerals': [],
      'poststart-sidecars': [],
      poststops: [],
      mains: [],
    };

    tasksOrStates.forEach((taskOrState) => {
      const task = taskOrState.task || taskOrState;

      if (task.lifecycleName) {
        lifecycles[`${task.lifecycleName}s`].push(taskOrState);
      }
    });

    const phases = [];
    const stateActiveIterator = (state) => state.state === 'running';

    if (lifecycles.mains.length < tasksOrStates.length) {
      phases.push({
        name: 'Prestart',
        isActive: lifecycles['prestart-ephemerals'].some(stateActiveIterator),
      });

      phases.push({
        name: 'Main',
        isActive:
          lifecycles.mains.some(stateActiveIterator) ||
          lifecycles['poststart-ephemerals'].some(stateActiveIterator),
      });

      // Poststart is rendered as a subphase of main and therefore has no independent active state
      phases.push({
        name: 'Poststart',
      });

      phases.push({
        name: 'Poststop',
        isActive: lifecycles.poststops.some(stateActiveIterator),
      });
    }

    return phases;
  }

  @sort('taskStates', function (a, b) {
    return getTaskSortPrefix(a.task).localeCompare(getTaskSortPrefix(b.task));
  })
  sortedLifecycleTaskStates;

  @sort('tasks', function (a, b) {
    return getTaskSortPrefix(a).localeCompare(getTaskSortPrefix(b));
  })
  sortedLifecycleTasks;
}

const lifecycleNameSortPrefix = {
  'prestart-ephemeral': 0,
  'prestart-sidecar': 1,
  main: 2,
  'poststart-sidecar': 3,
  'poststart-ephemeral': 4,
  poststop: 5,
};

function getTaskSortPrefix(task) {
  return `${lifecycleNameSortPrefix[task.lifecycleName]}-${task.name}`;
}
