/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

// @ts-check
import Component from '@glimmer/component';
import { alias } from '@ember/object/computed';
import { tracked } from '@glimmer/tracking';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

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
        .map((a) =>
          a
            .get('states')
            .map((s) => s.events.content)
            .flat()
        )
        .flat()
        .filter((a) => this.containsSearchTerm(a))
        .sort((a, b) => a.get('time') - b.get('time'))
        .reverse()
        .slice(0, MAX_NUMBER_OF_EVENTS);
    } catch (e) {
      this.triggerError(e);
      return [];
    }
  }

  @action triggerError(error) {
    this.errorState = error;
    this.notifications.add({
      title: 'Could not fetch deployment history',
      message: error,
      color: 'critical',
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
    return (
      taskEvent.message.toLowerCase().includes(this.searchTerm.toLowerCase()) ||
      taskEvent.type.toLowerCase().includes(this.searchTerm.toLowerCase()) ||
      taskEvent.state.allocation.shortId.includes(this.searchTerm.toLowerCase())
    );
  }

  // #endregion search
}
