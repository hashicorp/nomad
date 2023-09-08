/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { alias, readOnly } from '@ember/object/computed';
import Sortable from 'nomad-ui/mixins/sortable';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('boxed-section')
export default class Children extends Component.extend(Sortable) {
  @service system;
  @service userSettings;

  job = null;

  // Provide a value that is bound to a query param
  sortProperty = null;
  sortDescending = null;
  currentPage = null;

  @readOnly('userSettings.pageSize') pageSize;

  @computed('job.taskGroups.[]')
  get taskGroups() {
    return this.get('job.taskGroups') || [];
  }

  @computed('job.children.[]')
  get children() {
    return this.get('job.children') || [];
  }

  @alias('children') listToSort;
  @alias('listSorted') sortedChildren;

  resetPagination() {
    if (this.currentPage != null) {
      this.set('currentPage', 1);
    }
  }
}
