/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { action, computed } from '@ember/object';
import { filterBy, mapBy, or, sort } from '@ember/object/computed';
import generateExecUrl from 'nomad-ui/utils/generate-exec-url';
import openExecUrl from 'nomad-ui/utils/open-exec-url';
import classic from 'ember-classic-decorator';

@classic
export default class TaskGroupParent extends Component {
  @service router;

  @or('clickedOpen', 'currentRouteIsThisTaskGroup') isOpen;

  @computed('router.currentRoute', 'taskGroup.{job.name,name}')
  get currentRouteIsThisTaskGroup() {
    const route = this.router.currentRoute;

    if (route.name.includes('task-group')) {
      const taskGroupRoute = route.parent;
      const execRoute = taskGroupRoute.parent;

      return (
        execRoute.params.job_name === this.taskGroup.job.name &&
        taskGroupRoute.params.task_group_name === this.taskGroup.name
      );
    } else {
      return false;
    }
  }

  @computed('taskGroup.allocations.@each.clientStatus')
  get hasPendingAllocations() {
    return this.taskGroup.allocations.any(
      (allocation) => allocation.clientStatus === 'pending'
    );
  }

  @mapBy('taskGroup.allocations', 'states') allocationTaskStatesRecordArrays;
  @computed('allocationTaskStatesRecordArrays.[]')
  get allocationTaskStates() {
    const flattenRecordArrays = (accumulator, recordArray) =>
      accumulator.concat(recordArray.toArray());
    return this.allocationTaskStatesRecordArrays.reduce(
      flattenRecordArrays,
      []
    );
  }

  @filterBy('allocationTaskStates', 'isActive') activeTaskStates;

  @mapBy('activeTaskStates', 'task') activeTasks;
  @mapBy('activeTasks', 'taskGroup') activeTaskGroups;

  @computed(
    'activeTaskGroups.@each.name',
    'activeTaskStates.@each.name',
    'activeTasks.@each.name',
    'taskGroup.{name,tasks}'
  )
  get tasksWithRunningStates() {
    const activeTaskStateNames = this.activeTaskStates
      .filter((taskState) => {
        return (
          taskState.task &&
          taskState.task.taskGroup.name === this.taskGroup.name
        );
      })
      .mapBy('name');

    return this.taskGroup.tasks.filter((task) =>
      activeTaskStateNames.includes(task.name)
    );
  }

  taskSorting = ['name'];
  @sort('tasksWithRunningStates', 'taskSorting') sortedTasks;

  clickedOpen = false;

  @action
  toggleOpen() {
    this.toggleProperty('clickedOpen');
  }

  @action
  openInNewWindow(job, taskGroup, task) {
    let url = generateExecUrl(this.router, {
      job,
      taskGroup,
      task,
    });

    openExecUrl(url);
  }
}
