import Ember from 'ember';
import Component from '@glimmer/component';
import { task, timeout } from 'ember-concurrency';
import { assert } from '@ember/debug';
import { inject as service } from '@ember/service';
import { action, get } from '@ember/object';

export default class NodePrimaryMetric extends Component {
  @service token;
  @service('stats-trackers-registry') statsTrackersRegistry;

  /** Args
    allocation = null;
    metric null; (one of 'cpu' or 'memory'
  */

  get metric() {
    assert('metric is a required argument', this.args.metric);
    return this.args.metric;
  }

  get tracker() {
    return this.statsTrackersRegistry.getTracker(this.args.allocation);
  }

  get data() {
    if (!this.tracker) return [];
    return get(this, `tracker.${this.metric}`);
  }

  get series() {
    const ret = this.tracker.tasks
      .map(task => ({
        name: task.task,
        data: task[this.metric],
      }))
      .reverse();

    return ret;
  }

  get reservedAmount() {
    return this.metric === 'cpu' ? this.tracker.reservedCPU : this.tracker.reservedMemory;
  }

  get chartClass() {
    if (this.metric === 'cpu') return 'is-info';
    if (this.metric === 'memory') return 'is-danger';
    return 'is-primary';
  }

  get colorScale() {
    if (this.metric === 'cpu') return 'blues';
    if (this.metric === 'memory') return 'reds';
    return 'ordinal';
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
