/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { task } from 'ember-concurrency';
import { computed } from '@ember/object';

export default class JobRow extends Component {
  @service router;
  @service store;
  @service system;

  // @tracked fullActiveDeploymentObject = {};

  // /**
  //  * If our job has an activeDeploymentID, as determined by the statuses endpoint,
  //  * we check if this component's fullActiveDeploymentObject has the same ID.
  //  * If it does, we don't need to do any fetching: we can simply check this.fullActiveDeploymentObject.requiresPromotion
  //  * If it doesn't, we need to fetch the deployment with the activeDeploymentID
  //  * and set it to this.fullActiveDeploymentObject, then check this.fullActiveDeploymentObject.requiresPromotion.
  //  */
  // get requiresPromotion() {
  //   if (!this.args.job.hasActiveCanaries || !this.args.job.activeDeploymentID) {
  //     return false;
  //   }

  //   if (this.fullActiveDeploymentObject && this.fullActiveDeploymentObject.id === this.args.job.activeDeploymentID) {
  //     return this.fullActiveDeploymentObject.requiresPromotion;
  //   }

  //   this.fetchActiveDeployment();
  //   return false;
  // }

  // @action
  // async fetchActiveDeployment() {
  //   if (this.args.job.hasActiveCanaries && this.args.job.activeDeploymentID) {
  //     let deployment = await this.store.findRecord('deployment', this.args.job.activeDeploymentID);
  //     this.fullActiveDeploymentObject = deployment;
  //   }
  // }

  /**
   * Promotion of a deployment will error if the canary allocations are not of status "Healthy";
   * this function will check for that and disable the promote button if necessary.
   * @returns {boolean}
   */
  get canariesHealthy() {
    const relevantAllocs = this.args.job.allocations.filter(
      (a) => !a.isOld && a.isCanary && !a.hasBeenRescheduled
    );
    return (
      relevantAllocs.length &&
      relevantAllocs.every((a) => a.clientStatus === 'running' && a.isHealthy)
    );
  }

  /**
   * Similar to the below, but cares if any non-old canaries have failed, regardless of their rescheduled status.
   */
  get someCanariesHaveRescheduled() {
    // TODO: Weird thing where alloc.isUnhealthy right away, because alloc.DeploymentStatus.Healthy is false.
    // But that doesn't seem right: health check in that state should be unknown or null, perhaps.
    const relevantAllocs = this.args.job.allocations.filter(
      (a) => !a.isOld && a.isCanary
    );
    console.log(
      'relevantAllocs',
      relevantAllocs,
      relevantAllocs.map((a) => a.jobVersion),
      relevantAllocs.map((a) => a.clientStatus),
      relevantAllocs.map((a) => a.isUnhealthy)
    );

    return relevantAllocs.some(
      (a) =>
        a.clientStatus === 'failed' ||
        a.clientStatus === 'lost' ||
        a.isUnhealthy
    );
  }

  get someCanariesHaveFailedAndWontReschedule() {
    let availableSlotsToFill = this.args.job.expectedRunningAllocCount;
    let runningOrPendingCanaries = this.args.job.allocations.filter(
      (a) => !a.isOld && a.isCanary && !a.hasBeenRescheduled
    );
    const relevantAllocs = this.args.job.allocations.filter(
      (a) => !a.isOld && a.isCanary && !a.hasBeenRescheduled
    );
    console.log(
      'relevantAllocs',
      relevantAllocs,
      relevantAllocs.map((a) => a.jobVersion),
      relevantAllocs.map((a) => a.clientStatus),
      relevantAllocs.map((a) => a.isUnhealthy)
    );
    return relevantAllocs.some(
      (a) =>
        a.clientStatus === 'failed' ||
        a.clientStatus === 'lost' ||
        a.isUnhealthy
    );
  }

  @task(function* () {
    // ID: jobDeployments[0]?.id,
    // IsActive: jobDeployments[0]?.status === 'running',
    // // IsActive: true,
    // JobVersion: jobDeployments[0]?.versionNumber,
    // Status: jobDeployments[0]?.status,
    // StatusDescription: jobDeployments[0]?.statusDescription,
    // AllAutoPromote: false,
    // RequiresPromotion: true, // TODO: lever

    /**
     * @typedef DeploymentSummary
     * @property {string} id
     * @property {boolean} isActive
     * @property {string} jobVersion
     * @property {string} status
     * @property {string} statusDescription
     * @property {boolean} allAutoPromote
     * @property {boolean} requiresPromotion
     */
    /**
     * @type {DeploymentSummary}
     */
    let latestDeploymentSummary = this.args.job.latestDeploymentSummary;

    // console.log(
    //   'checking if requries promotion',
    //   this.args.job.name,
    //   latestDeploymentSummary,
    //   this.args.job.hasActiveCanaries
    // );
    // Early return false if we don't have an active deployment
    if (!latestDeploymentSummary.isActive) {
      return false;
    }

    // Early return if we our deployment doesn't have any canaries
    if (!this.args.job.hasActiveCanaries) {
      console.log('!hasActiveCan');
      return false;
    }

    if (latestDeploymentSummary.requiresPromotion) {
      console.log('requires promotion, and...');
      if (this.canariesHealthy) {
        console.log('canaries are healthy.');
        return 'canary-promote';
      }
      // if (this.someCanariesHaveFailedAndWontReschedule) {
      if (this.someCanariesHaveRescheduled) {
        // TODO: I'm uncertain about when to alert the user here. It seems like it might be important
        // enough to let them know when ANY canary has to be rescheduled, but there's an argument to be
        // made that we oughtn't bother them until it's un-reschedulable.
        console.log('some canaries have failed.');
        return 'canary-failure';
      }
      if (latestDeploymentSummary.allAutoPromote) {
        console.log(
          'This deployment is set to auto-promote; canaries are being checked now'
        );
        // return "This deployment is set to auto-promote; canaries are being checked now";
        return false;
      } else {
        console.log(
          'This deployment requires manual promotion and things are being checked now'
        );
        // return "This deployment requires manual promotion and things are being checked now";
        return false;
      }
    }
    return false;
  })
  requiresPromotionTask;

  @task(function* () {
    try {
      yield this.args.job.latestDeployment.content.promote(); // TODO: need to do a deployment findRecord here first.
      // dont bubble up
      return false;
    } catch (err) {
      // TODO: handle error. add notifications.
      console.log('caught error', err);
      // this.handleError({
      //   title: 'Could Not Promote Deployment',
      //   // description: messageFromAdapterError(err, 'promote deployments'),
      // });

      // err.errors.forEach((err) => {
      //   this.notifications.add({
      //     title: "Could not promote deployment",
      //     message: err.detail,
      //     color: 'critical',
      //     timeout: 8000,
      //   });
      // });
    }
  })
  promote;

  get latestDeploymentFailed() {
    return this.args.job.latestDeploymentSummary.status === 'failed';
  }

  @action
  gotoJob() {
    const { job } = this.args;
    this.router.transitionTo('jobs.job.index', job.idWithNamespace);
  }
}
