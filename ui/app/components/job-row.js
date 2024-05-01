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

export default class JobRow extends Component {
  @service router;
  @service store;
  @service system;

  @tracked activeDeployment = null;

  // /**
  //  * If our job has an activeDeploymentID, as determined by the statuses endpoint,
  //  * we check if this component's activeDeployment has the same ID.
  //  * If it does, we don't need to do any fetching: we can simply check this.activeDeployment.requiresPromotion
  //  * If it doesn't, we need to fetch the deployment with the activeDeploymentID
  //  * and set it to this.activeDeployment, then check this.activeDeployment.requiresPromotion.
  //  */
  // get requiresPromotion() {
  //   if (!this.args.job.hasActiveCanaries || !this.args.job.activeDeploymentID) {
  //     return false;
  //   }

  //   if (this.activeDeployment && this.activeDeployment.id === this.args.job.activeDeploymentID) {
  //     return this.activeDeployment.requiresPromotion;
  //   }

  //   this.fetchActiveDeployment();
  //   return false;
  // }

  // @action
  // async fetchActiveDeployment() {
  //   if (this.args.job.hasActiveCanaries && this.args.job.activeDeploymentID) {
  //     let deployment = await this.store.findRecord('deployment', this.args.job.activeDeploymentID);
  //     this.activeDeployment = deployment;
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

  get someCanariesHaveFailed() {
    const relevantAllocs = this.args.job.allocations.filter(
      (a) => !a.isOld && a.isCanary && !a.hasBeenRescheduled
    );
    console.log(
      'relevantAllocs',
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
    console.log(
      'checking if requries promotion',
      this.args.job.name,
      this.args.job.latestDeploymentSummary,
      this.args.job.hasActiveCanaries
    );
    if (
      !this.args.job.hasActiveCanaries ||
      !this.args.job.latestDeploymentSummary?.IsActive
    ) {
      return false;
    }

    if (
      !this.latestDeploymentSummary?.IsActive ||
      this.activeDeployment.id !== this.args.job?.latestDeploymentSummary.ID
    ) {
      this.activeDeployment = yield this.store.findRecord(
        'deployment',
        this.args.job.latestDeploymentSummary.ID
      );
    }

    if (this.activeDeployment.requiresPromotion) {
      if (this.canariesHealthy) {
        return 'canary-promote';
      }
      if (this.someCanariesHaveFailed) {
        return 'canary-failure';
      }
      if (this.activeDeployment.isAutoPromoted) {
        // return "This deployment is set to auto-promote; canaries are being checked now";
        return false;
      } else {
        // return "This deployment requires manual promotion and things are being checked now";
        return false;
      }
    }
    return false;
  })
  requiresPromotionTask;

  @task(function* () {
    try {
      yield this.args.job.latestDeployment.content.promote();
      // dont bubble up
      return false;
    } catch (err) {
      this.handleError({
        title: 'Could Not Promote Deployment',
        // description: messageFromAdapterError(err, 'promote deployments'),
      });
    }
  })
  promote;

  /**
   * If there is not a deployment happening,
   * and the running allocations have a jobVersion that differs from the job's version,
   * we can assume a failed latest deployment.
   */
  get latestDeploymentFailed() {
    /**
     * Import from app/models/job.js
     * @type {import('../models/job').default}
     */
    const job = this.args.job;
    if (job.latestDeploymentSummary?.IsActive) {
      return false;
    }

    // We only want to show this status if the job is running, to indicate to
    // the user that the job is not running the version they expect given their
    // latest deployment.
    if (
      !(
        job.aggregateAllocStatus.label === 'Healthy' ||
        job.aggregateAllocStatus.label === 'Degraded' ||
        job.aggregateAllocStatus.label === 'Recovering'
      )
    ) {
      return false;
    }
    const runningAllocs = job.allocations.filter(
      (a) => a.clientStatus === 'running'
    );
    const jobVersion = job.version;
    return runningAllocs.some((a) => a.jobVersion !== jobVersion);
  }

  @action
  gotoJob() {
    const { job } = this.args;
    this.router.transitionTo('jobs.job.index', job.idWithNamespace);
  }
}
