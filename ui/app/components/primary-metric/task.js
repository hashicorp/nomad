import Ember from 'ember';
import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
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

  @tracked tracker = null;
  @tracked taskState = null;

  get metric() {
    assert('metric is a required argument', this.args.metric);
    return this.args.metric;
  }

  get data() {
    if (!this.tracker) return [];
    const task = this.tracker.tasks.findBy('task', this.taskState.name);
    return task && task[this.metric];
  }

  get reservedAmount() {
    const { cpu, memory } = this.args.taskState.allocation.allocatedResources;
    if (this.metric === 'cpu') return cpu;
    if (this.metric === 'memory') return memory;
    return null;
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
    this.taskState = this.args.taskState;
    this.tracker = this.statsTrackersRegistry.getTracker(this.args.taskState.allocation);
    this.poller.perform();
  }

  willDestroy() {
    this.poller.cancelAll();
    this.tracker.signalPause.perform();
  }
}
