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
// import * as d3 from 'd3';
import d3Scale from 'd3-scale';
// import { computed } from '@ember/object';
const MAX_NUMBER_OF_EVENTS = 500;

const EVENT_PAIRS = {
  'Task Setup': 'Started',
  Restarting: 'Started',
  Killing: 'Killed',
  'Downloading Artifacts': 'Started',
};

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
    console.log('deploymentAllocations', this.args.allocations);
    return (
      this.args.allocations ||
      this.jobAllocations.filter(
        (alloc) => alloc.jobVersion === this.deploymentVersion
      )
    );
  }

  get allocationEvents() {
    try {
      let events = this.deploymentAllocations
        .map((a) =>
          a
            .get('states')
            .map((s) => s.events.content)
            .flat()
        )
        .flat();
      return events.sort((a, b) => a.get('time') - b.get('time'));
    } catch (e) {
      this.triggerError(e);
      return [];
    }
  }

  // #region waterfall
  get earliestStartTime() {
    console.count('earliestStartTime');
    let startTimes = this.allocationEvents.map((e) => e.get('time'));
    startTimes = startTimes.filter((t) => !!t);
    return Math.min(...startTimes);
  }

  get latestStartTime() {
    console.count('latestStartTime');
    let startTimes = this.allocationEvents.map((e) => e.get('time'));
    startTimes = startTimes.filter((t) => !!t);
    return Math.max(...startTimes);
  }

  get runningMinutes() {
    console.log('running minutes recompute');
    // return Math.round((this.currentTime - this.earliestStartTime) / 60000);
    return Math.round(
      (this.latestStartTime - this.earliestStartTime) / 1000 / 60
    );
  }
  // #endregion waterfall

  get searchedAllocationEvents() {
    return this.allocationEvents.filter((e) => this.containsSearchTerm(e));
  }

  /**
   * Map the deployment's allocations to their task events, in reverse-chronological order
   * @type { import('../../models/task-event').default[] }
   */
  get history() {
    try {
      console.count('history recompute');
      let timeScale = d3Scale
        .scaleLinear()
        .domain([this.earliestStartTime, this.latestStartTime])
        .range([0, 100]);

      let events = this.searchedAllocationEvents.map((e) => {
        let relativeTime = timeScale(e.get('time'));
        e.relativeTime = relativeTime;
        return e;
      });

      // A (maybe inefficient but safe?) second pass to calculate distance for paired events
      events.forEach((event, index) => {
        const endEventType = EVENT_PAIRS[event.type];
        if (endEventType) {
          // Find the next matching event with same allocation ID
          const nextEvent = events
            .slice(index + 1)
            .find(
              (e) =>
                e.type === endEventType &&
                e.state.allocation.id === event.state.allocation.id
            );

          if (nextEvent) {
            // console.log('nextEvent', nextEvent);
            const nextEventRelativeTime = nextEvent.relativeTime;
            const eventRelativeTime = event.relativeTime;
            const length = nextEventRelativeTime - eventRelativeTime;
            event.relativeLength = length;
            event.nextEventTime = nextEvent.get('time');
            console.log('length', length);
          }
        }
      });

      events = events.reverse().slice(0, MAX_NUMBER_OF_EVENTS);
      return events;
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
