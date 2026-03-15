/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { alias } from '@ember/object/computed';
import { tracked } from '@glimmer/tracking';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { scheduleOnce } from '@ember/runloop';

const MAX_NUMBER_OF_EVENTS = 500;

export default class JobStatusDeploymentHistoryComponent extends Component {
  @service notifications;

  @tracked isHidden = this.args.isHidden;

  /**
   * @type { Error }
   */
  @tracked errorState = null;

  /**
   * @type { import('../../models/job').default }
   */
  @alias('args.deployment.job') job;

  /**
   * @type { number }
   */
  @alias('args.deployment.versionNumber') deploymentVersion;

  /**
   * Get all allocations for the job
   * @type { import('../../models/allocation').default[] }
   */
  get jobAllocations() {
    return this.job.get('allocations');
  }

  /**
   * Filter the job's allocations to only those that are part of the deployment
   * @type { import('../../models/allocation').default[] }
   */
  get deploymentAllocations() {
    return (
      this.args.allocations ||
      this.jobAllocations.filter(
        (alloc) => alloc.jobVersion === this.deploymentVersion
      )
    );
  }

  /**
   * Map the deployment's allocations to their task events, in reverse-chronological order
   * @type { import('../../models/task-event').default[] }
   */
  get history() {
    try {
      return this.deploymentAllocations
        .map((allocation) => {
          const states = allocation?.get?.('states') || allocation?.states || [];
          const stateList = states?.toArray?.() || states || [];

          return stateList
            .map((state) => state?.events?.toArray?.() || state?.events || [])
            .flat();
        })
        .flat()
        .filter(Boolean)
        .filter((taskEvent) => this.containsSearchTerm(taskEvent))
        .sort((a, b) => {
          const aTime = a?.time?.valueOf?.() || a?.get?.('time') || 0;
          const bTime = b?.time?.valueOf?.() || b?.get?.('time') || 0;
          return aTime - bTime;
        })
        .reverse()
        .slice(0, MAX_NUMBER_OF_EVENTS);
    } catch (e) {
      this.triggerError(e);
      return [];
    }
  }

  @action triggerError(error) {
    scheduleOnce('actions', this, () => {
      if (this.errorState === error) {
        return;
      }

      this.errorState = error;
      this.notifications.add({
        title: 'Could not fetch deployment history',
        message: error?.message || String(error),
        color: 'critical',
      });
    });
  }

  // #region search

  /**
   * @type { string }
   */
  @tracked searchTerm = '';

  /**
   * @param { import('../../models/task-event').default } taskEvent
   * @returns { boolean }
   */
  containsSearchTerm(taskEvent) {
    if (!taskEvent) {
      return false;
    }

    const message = (taskEvent.message || '').toLowerCase();
    const type = (taskEvent.type || '').toLowerCase();
    const allocationShortId =
      taskEvent.state?.allocation?.shortId?.toLowerCase?.() || '';

    return (
      message.includes(this.searchTerm.toLowerCase()) ||
      type.includes(this.searchTerm.toLowerCase()) ||
      allocationShortId.includes(this.searchTerm.toLowerCase())
    );
  }

  // #endregion search
}
