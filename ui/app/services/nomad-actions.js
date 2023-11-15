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
    this.flyoutActive = true;
  }
  @action closeFlyout() {
    this.flyoutActive = false;
  }

  /**
   * @type {import('../models/action-instance').default[]}
   */
  @tracked
  actionsQueue = [];

  updateQueue() {
    this.actionsQueue = [...this.actionsQueue];
  }

  /**
   *
   * @param {import("../models/action").default} action
   * @param {string} allocID
   * @param {import("../models/job").default} job
   */
  async runAction(action, allocID, job) {
    console.log('service running action', action, allocID, job);
    const actionQueueID = `${action.name}-${allocID}-${Date.now()}`;
    /**
     * @type {import ('../models/action-instance').default}
     */
    const actionInstance = await this.store.createRecord('action-instance', {
      state: 'pending',
      id: actionQueueID,
      allocID,
    });

    // Note: setting post-createRecord because of a noticed bug
    // when passing action as a property to createRecord.
    actionInstance.set('action', action);

    job.runAction(action, allocID, actionInstance);

    this.actionsQueue.unshift(actionInstance); // add to the front of the queue
    this.updateQueue();
    this.openFlyout();
  }

  /**
   *
   * @param {import ('../models/action-instance').default} actionInstance
   */
  @action clearActionInstance(actionInstance) {
    // if instance is still running, stop it
    if (actionInstance.state === 'running') {
      actionInstance.socket.close();
    }
    this.actionsQueue = this.actionsQueue.filter(
      (a) => a.id !== actionInstance.id
    );
    // this.updateQueue();
  }

  @action clearFinishedActions() {
    // this.actionsQueue = [];
    this.actionsQueue = this.actionsQueue.filter((a) => a.state !== 'complete');
  }

  @action stopAll() {
    this.actionsQueue.forEach((a) => {
      if (a.state === 'running') {
        a.socket.close();
      }
    });
  }

  get runningActions() {
    return this.actionsQueue.filter((a) => a.state === 'running');
  }

  get finishedActions() {
    return this.actionsQueue.filter(
      (a) => a.state === 'complete' || a.state === 'error'
    );
  }
}
