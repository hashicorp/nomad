/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import Sortable from 'nomad-ui/mixins/sortable';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('boxed-section')
export default class TaskGroups extends Component.extend(Sortable) {
  job = null;

  // Provide a value that is bound to a query param
  sortProperty = null;
  sortDescending = null;

  @computed('job.taskGroups.[]')
  get taskGroups() {
    return this.get('job.taskGroups') || [];
  }

  @alias('taskGroups') listToSort;
  @alias('listSorted') sortedTaskGroups;
}
