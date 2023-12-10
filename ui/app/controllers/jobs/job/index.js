/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Controller from '@ember/controller';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';
import WithNamespaceResetting from 'nomad-ui/mixins/with-namespace-resetting';
import classic from 'ember-classic-decorator';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
@classic
export default class IndexController extends Controller.extend(
  WithNamespaceResetting
) {
  @service system;

  queryParams = [
    {
      currentPage: 'page',
    },
    {
      sortProperty: 'sort',
    },
    {
      sortDescending: 'desc',
    },
    'activeTask',
    'statusMode',
  ];

  currentPage = 1;

  @alias('model') job;

  sortProperty = 'name';
  sortDescending = false;

  @tracked activeTask = null;

  /**
   * @type {('current'|'historical')}
   */
  @tracked
  statusMode = 'current';

  @action
  setActiveTaskQueryParam(task) {
    if (task) {
      this.activeTask = `${task.allocation.id}-${task.name}`;
    } else {
      this.activeTask = null;
    }
  }

  /**
   * @param {('current'|'historical')} mode
   */
  @action
  setStatusMode(mode) {
    this.statusMode = mode;
  }
}
