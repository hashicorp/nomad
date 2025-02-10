/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';

export default class ActionsFlyoutComponent extends Component {
  @service nomadActions;
  @service router;

  get job() {
    if (this.task) {
      return this.task.taskGroup.job;
    } else {
      return (
        this.router.currentRouteName.startsWith('jobs.job') &&
        this.router.currentRoute.attributes
      );
    }
  }

  get task() {
    return (
      this.router.currentRouteName.startsWith('allocations.allocation.task') &&
      this.router.currentRoute.attributes.task
    );
  }

  get allocation() {
    return (
      this.args.allocation ||
      (this.task && this.router.currentRoute.attributes.allocation)
    );
  }

  get contextualParent() {
    return this.task || this.job;
  }

  get contextualActions() {
    return this.contextualParent?.actions || [];
  }

  @alias('nomadActions.flyoutActive') isOpen;

  /**
   * Group peers together by their peerID
   */
  get actionInstances() {
    let instances = this.nomadActions.actionsQueue;

    // Only keep the first of any found peerID value from the list
    let peerIDs = new Set();
    let filteredInstances = [];
    for (let instance of instances) {
      if (!instance.peerID || !peerIDs.has(instance.peerID)) {
        filteredInstances.push(instance);
        peerIDs.add(instance.peerID);
      }
    }

    return filteredInstances;
  }
}
