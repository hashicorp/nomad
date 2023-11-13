/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// Guess who just found out that "actions" is a reserved name in Ember?
// Signed, the person who just renamed this NomadActions.

// @ts-check
import Service from '@ember/service';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
// import ActionModel from '../models/action';
// import JobModel from '../models/job';
// import ActionInstanceModel from '../models/action-instance';
import { action } from '@ember/object';

// /**
//  * @typedef ActionObject
//  * @property {"running"|"complete"} state
//  * @property {string} id
//  * @property {ActionModel} action
//  */

export default class NomadActionsService extends Service {
  @service can;
  @service store;

  // Note: future Actions Governance work (https://github.com/hashicorp/nomad/issues/18800)
  // will require this to be a computed property that depends on the current user's permissions.
  // For now, we simply check alloc exec privileges.
  get hasActionPermissions() {
    return this.can.can('exec allocation');
  }

  @tracked flyoutActive = false;

  @action openFlyout() {
    console.log('opening flyout', this.flyoutActive);
    this.flyoutActive = true;
  }
  @action closeFlyout() {
    console.log('closing flyout', this.flyoutActive);
    this.flyoutActive = false;
  }

  /**
   * @type {import('../models/action-instance').default[]}
   */
  @tracked
  actionsQueue = [];

  updateQueue() {
    console.log('updating queue');
    this.actionsQueue = [...this.actionsQueue];
  }

  /**
   *
   * @param {import("../models/action").default} action
   * @param {string} allocID
   * @param {import("../models/job").default} job
   */
  runAction(action, allocID, job) {
    const actionQueueID = `${action.name}-${Date.now()}`;
    /**
     * @type {import ('../models/action-instance').default}
     */
    const actionInstance = this.store.createRecord('action-instance', {
      action,
      state: 'pending',
      id: actionQueueID,
    });

    job.runAction(action, allocID, actionInstance);
    this.actionsQueue.push(actionInstance);
    this.updateQueue();

    this.openFlyout();
  }
}
