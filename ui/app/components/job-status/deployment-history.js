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
import moment from 'moment';

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

  // Helper function for pluralization
  pluralize(count, singular, plural) {
    return `${count} ${count === 1 ? singular : plural} ago`;
  }

  // Function to dynamically determine the period
  dynamicPeriod(timestamp, currentTime) {
    const diffMinutes = currentTime.diff(moment(timestamp), 'minutes');
    if (diffMinutes < 60)
      return this.pluralize(diffMinutes, 'minute', 'minutes');
    const diffHours = currentTime.diff(moment(timestamp), 'hours');
    if (diffHours < 24) return this.pluralize(diffHours, 'hour', 'hours');
    const diffDays = currentTime.diff(moment(timestamp), 'days');
    if (diffDays < 7) return this.pluralize(diffDays, 'day', 'days');
    const diffWeeks = currentTime.diff(moment(timestamp), 'weeks');
    return this.pluralize(diffWeeks, 'week', 'weeks');
  }

  calculateAverageDistance = (groupedEvents) => {
    // Ensure there are enough events to calculate distances
    if (groupedEvents.length < 2) return 0;

    // Calculate distances between consecutive medians
    let totalDistance = 0;
    for (let i = 1; i < groupedEvents.length; i++) {
      const previousMedian = moment(groupedEvents[i - 1].median); // Previous group's median
      const currentMedian = moment(groupedEvents[i].median); // Current group's median
      const distance = currentMedian.diff(previousMedian, 'seconds');
      totalDistance += distance;
    }

    // Calculate and return the average distance
    return totalDistance / (groupedEvents.length - 1);
  };

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

  get categorizedHistory() {
    let history = this.history;
    console.log('history is', history);
    const now = moment();

    // Group into dynamic periods
    let groupedEvents = history
      .reduce((acc, event) => {
        const timestamp = event.get('time');
        let period = this.dynamicPeriod(timestamp, now);
        if (period === '0 minutes ago') {
          period = 'Just now';
        }
        const index = acc.findIndex((group) => group.period === period);
        if (index > -1) {
          acc[index].events.push(timestamp);
          acc[index].realevents.push(event);
        } else {
          acc.push({ period, events: [timestamp], realevents: [event] });
        }
        return acc;
      }, [])
      // map over them to get median time, expresssed as a javascript date object
      .map((group) => {
        const median = new Date(
          group.events.reduce(
            (a, b) => new Date(a).getTime() + new Date(b).getTime(),
            0
          ) / group.events.length
        );
        return {
          period: group.period,
          median,
          events: group.events,
          realEvents: group.realevents,
        };
      })
      .sort((a, b) => a.median - b.median);

    console.log('groupedEvents', groupedEvents);

    // Calculate the average distance between periods
    const averageNeighbourDistance =
      this.calculateAverageDistance(groupedEvents);
    console.log('===================');
    console.log('average distance', averageNeighbourDistance);

    // Group events by proximity

    // If the distance between any 2 periods is less than 20% of the median distance, group them together
    // Do this recursively until no two groups meet that criteria
    let threshold = averageNeighbourDistance * 0.2;
    let proximateGroupedEvents = groupedEvents.reduce(
      (acc, group, index, array) => {
        const previousGroup = array[index - 1];
        if (previousGroup) {
          const distance = moment(group.median).diff(
            moment(previousGroup.median),
            'seconds'
          );
          console.log(
            '  - distance between',
            group.period,
            'and',
            previousGroup.period,
            'is',
            distance,
            'seconds'
          );
          if (distance < threshold) {
            previousGroup.period = `${previousGroup.period} (demo note: clustered!)`;
            previousGroup.events = previousGroup.events.concat(group.events);
            previousGroup.median = new Date(
              previousGroup.events.reduce(
                (a, b) => new Date(a).getTime() + new Date(b).getTime(),
                0
              ) / previousGroup.events.length
            );
          } else {
            acc.push(group);
          }
        } else {
          acc.push(group);
        }
        return acc;
      },
      []
    );
    console.log('proximateGroupedEvents', proximateGroupedEvents);

    return proximateGroupedEvents.reverse();
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

  @action downloadHistory() {
    const csvContent = this.history
      .map((event) => {
        return `${event.time},${event.type},${event.message},${event.state.allocation.shortId}`;
      })
      .join('\n');

    const blob = new Blob([csvContent], {
      type: 'text/plain',
    });
    const url = window.URL.createObjectURL(blob);
    const downloadAnchor = document.createElement('a');

    downloadAnchor.href = url;
    downloadAnchor.target = '_blank';
    downloadAnchor.rel = 'noopener noreferrer';
    downloadAnchor.download = 'allocation_history.csv';

    downloadAnchor.click();
    downloadAnchor.remove();

    window.URL.revokeObjectURL(url);
    this.notifications.add({
      title: 'Downloaded!',
      color: 'success',
      icon: 'download',
    });
  }
}
