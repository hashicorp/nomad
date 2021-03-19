import Ember from 'ember';
import Component from '@glimmer/component';
import { task, timeout } from 'ember-concurrency';
import { assert } from '@ember/debug';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class TaskPrimaryMetric extends Component {
  @service('stats-trackers-registry') statsTrackersRegistry;

  /** Args
    taskState = null;
    metric null; (one of 'cpu' or 'memory'
  */

  get metric() {
    assert('metric is a required argument', this.args.metric);
    return this.args.metric;
  }

  get tracker() {
    return this.statsTrackersRegistry.getTracker(this.args.taskState.allocation);
  }

  get data() {
    if (!this.tracker) return [];
    const task = this.tracker.tasks.findBy('task', this.args.taskState.name);
    return task && task[this.metric];
  }

  get reservedAmount() {
    const task = this.tracker.tasks.findBy('task', this.args.taskState.name);
    return this.metric === 'cpu' ? task.reservedCPU : task.reservedMemory;
  }

  get chartClass() {
    if (this.metric === 'cpu') return 'is-info';
    if (this.metric === 'memory') return 'is-danger';
    return 'is-primary';
  }

  @task(function*() {
    do {
      this.tracker.poll.perform();
      yield timeout(100);
    } while (!Ember.testing);
  })
  poller;

  @action
  start() {
    if (this.tracker) this.poller.perform();
  }

  willDestroy() {
    this.poller.cancelAll();
    this.tracker.signalPause.perform();
  }
}
