/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Ember from 'ember';
import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import { task, timeout } from 'ember-concurrency';
import { tracked } from '@glimmer/tracking';

export default class TaskSubRowComponent extends Component {
  @service store;
  @service router;
  @service('stats-trackers-registry') statsTrackersRegistry;

  constructor() {
    super(...arguments);
    // Kick off stats polling
    const allocation = this.task.allocation;
    if (allocation) {
      this.fetchStats.perform();
    } else {
      this.fetchStats.cancelAll();
    }
  }

  @alias('args.taskState') task;

  @action
  gotoTask(allocation, task) {
    this.router.transitionTo('allocations.allocation.task', allocation, task);
  }

  // Since all tasks for an allocation share the same tracker, use the registry
  @computed('task.{allocation,isRunning}')
  get stats() {
    if (!this.task.isRunning) return undefined;

    return this.statsTrackersRegistry.getTracker(this.task.allocation);
  }

  // Internal state
  @tracked statsError = false;

  @computed
  get enablePolling() {
    return !Ember.testing;
  }

  @computed('task.name', 'stats.tasks.[]')
  get taskStats() {
    if (!this.stats) return undefined;

    return this.stats.tasks.findBy('task', this.task.name);
  }

  @alias('taskStats.cpu.lastObject') cpu;
  @alias('taskStats.memory.lastObject') memory;

  @(task(function* () {
    do {
      if (this.stats) {
        try {
          yield this.stats.poll.linked().perform();
          this.statsError = false;
        } catch (error) {
          this.statsError = true;
        }
      }

      yield timeout(500);
    } while (this.enablePolling);
  }).drop())
  fetchStats;

  //#region Logs Sidebar

  @alias('args.active') shouldShowLogs;

  @action handleTaskLogsClick(task) {
    if (this.args.onSetActiveTask) {
      this.args.onSetActiveTask(task);
    }
  }

  @action closeSidebar() {
    if (this.args.onSetActiveTask) {
      this.args.onSetActiveTask(null);
    }
  }

  //#endregion Logs Sidebar
}
