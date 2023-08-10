/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Ember from 'ember';
import Component from '@glimmer/component';
import { task, timeout } from 'ember-concurrency';
import { assert } from '@ember/debug';
import { inject as service } from '@ember/service';
import { action, get } from '@ember/object';
import {
  formatScheduledBytes,
  formatScheduledHertz,
} from 'nomad-ui/utils/units';

export default class NodePrimaryMetric extends Component {
  @service('stats-trackers-registry') statsTrackersRegistry;

  /** Args
    node = null;
    metric null; (one of 'cpu' or 'memory')
  */

  get metric() {
    assert('metric is a required argument', this.args.metric);
    return this.args.metric;
  }

  get tracker() {
    return this.statsTrackersRegistry.getTracker(this.args.node);
  }

  get data() {
    if (!this.tracker) return [];
    return get(this, `tracker.${this.metric}`);
  }

  get reservedAmount() {
    if (this.metric === 'cpu') return this.tracker.reservedCPU;
    if (this.metric === 'memory') return this.tracker.reservedMemory;
    return null;
  }

  get chartClass() {
    if (this.metric === 'cpu') return 'is-info';
    if (this.metric === 'memory') return 'is-danger';
    return 'is-primary';
  }

  get reservedAnnotations() {
    if (this.metric === 'cpu' && get(this.args.node, 'reserved.cpu')) {
      const cpu = this.args.node.reserved.cpu;
      return [
        {
          label: `${formatScheduledHertz(cpu, 'MHz')} reserved`,
          percent: cpu / this.reservedAmount,
        },
      ];
    }

    if (this.metric === 'memory' && get(this.args.node, 'reserved.memory')) {
      const memory = this.args.node.reserved.memory;
      return [
        {
          label: `${formatScheduledBytes(memory, 'MiB')} reserved`,
          percent: memory / this.reservedAmount,
        },
      ];
    }

    return [];
  }

  @task(function* () {
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
    super.willDestroy(...arguments);
    this.poller.cancelAll();
    this.tracker.signalPause.perform();
  }
}
