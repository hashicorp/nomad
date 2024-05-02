/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { task } from 'ember-concurrency';

export default class JobRow extends Component {
  @service router;
  @service store;
  @service system;

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
   * Used to inform the user that an allocation has entered into a perment state of failure:
   * That is, it has exhausted its restarts and its reschedules and is in a terminal state.
   */
  get someCanariesHaveFailedAndWontReschedule() {
    const relevantAllocs = this.args.job.allocations.filter(
      (a) => !a.isOld && a.isCanary && !a.hasBeenRescheduled
    );

    return relevantAllocs.some(
      (a) =>
        a.clientStatus === 'failed' ||
        a.clientStatus === 'lost' ||
        a.isUnhealthy
    );
  }

  // eslint-disable-next-line require-yield
  @task(function* () {
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

      if (this.someCanariesHaveFailedAndWontReschedule) {
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
