/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Ember from 'ember';
import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import { task, timeout } from 'ember-concurrency';
import { lazyClick } from '../helpers/lazy-click';

import {
  classNames,
  tagName,
  attributeBindings,
} from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('tr')
@classNames('task-row', 'is-interactive')
@attributeBindings('data-test-task-row')
export default class TaskRow extends Component {
  @service store;
  @service token;
  @service('stats-trackers-registry') statsTrackersRegistry;

  task = null;

  // Internal state
  statsError = false;

  @computed
  get enablePolling() {
    return !Ember.testing;
  }

  // Since all tasks for an allocation share the same tracker, use the registry
  @computed('task.{allocation,isRunning}')
  get stats() {
    if (!this.get('task.isRunning')) return undefined;

    return this.statsTrackersRegistry.getTracker(this.get('task.allocation'));
  }

  @computed('task.name', 'stats.tasks.[]')
  get taskStats() {
    if (!this.stats) return undefined;

    return this.get('stats.tasks').findBy('task', this.get('task.name'));
  }

  @alias('taskStats.cpu.lastObject') cpu;
  @alias('taskStats.memory.lastObject') memory;

  onClick() {}

  click(event) {
    lazyClick([this.onClick, event]);
  }

  @(task(function* () {
    do {
      if (this.stats) {
        try {
          yield this.get('stats.poll').linked().perform();
          this.set('statsError', false);
        } catch (error) {
          this.set('statsError', true);
        }
      }

      yield timeout(500);
    } while (this.enablePolling);
  }).drop())
  fetchStats;

  didReceiveAttrs() {
    super.didReceiveAttrs();
    const allocation = this.get('task.allocation');

    if (allocation) {
      this.fetchStats.perform();
    } else {
      this.fetchStats.cancelAll();
    }
  }
}
