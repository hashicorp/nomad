/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { action, computed } from '@ember/object';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import Sortable from 'nomad-ui/mixins/sortable';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('boxed-section')
export default class TaskGroups extends Component.extend(Sortable) {
  @service router;

  job = null;

  // Provide a value that is bound to a query param
  sortProperty = null;
  sortDescending = null;

  @action
  gotoTaskGroup(taskGroup) {
    this.router.transitionTo('jobs.job.task-group', this.job, taskGroup.name);
  }

  @computed('job.taskGroups.[]')
  get taskGroups() {
    return this.get('job.taskGroups') || [];
  }

  @alias('taskGroups') listToSort;
  @alias('listSorted') sortedTaskGroups;
}
