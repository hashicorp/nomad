/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// Guess who just found out that "actions" is a reserved name in Ember?
// Signed, the person who just renamed this NomadActions.

// TODO: Move a lot of the job adapter funcs to here

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
   * @typedef {Object} RunActionParams
   * @property {import("../models/action").default} action
   * @property {string} allocID
   * @property {string} [peerID]
   */

  /**
   * @param {RunActionParams} params
   */
  @action runAction({ action, allocID, peerID = null }) {
    const job = action.task.taskGroup.job;

    const actionQueueID = `${action.name}-${allocID}-${Date.now()}`;
    /**
     * @type {import ('../models/action-instance').default}
     */
    const actionInstance = this.store.createRecord('action-instance', {
      state: 'pending',
      id: actionQueueID,
      allocID,
      peerID,
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
   * @param {import('../models/action').default} action
   */
  @action runActionOnRandomAlloc(action) {
    let allocID =
      action.allocations[Math.floor(Math.random() * action.allocations.length)]
        .id;
    this.runAction({ action, allocID });
  }

  /**
   * @param {import('../models/action').default} action
   */
  @action runActionOnAllAllocs(action) {
    // Generate a new peer ID for these action instances to use
    const peerID = `${action.name}-${Date.now()}`;
    action.allocations.forEach((alloc) => {
      this.runAction({ action, allocID: alloc.id, peerID });
    });
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

    // If action had peers, clear them out as well
    if (actionInstance.peerID) {
      this.actionsQueue = this.actionsQueue.filter(
        (a) => a.peerID !== actionInstance.peerID
      );
    }
    this.updateQueue();
  }

  @action clearFinishedActions() {
    // this.actionsQueue = [];
    this.actionsQueue = this.actionsQueue.filter((a) => a.state !== 'complete');
  }

  @action stopAll(peerID = null) {
    let actionsToStop = this.actionsQueue;
    if (peerID) {
      actionsToStop = actionsToStop.filter((a) => a.peerID === peerID);
    }
    actionsToStop.forEach((a) => {
      if (a.state === 'running') {
        a.socket.close();
      }
    });
    this.updateQueue();
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
