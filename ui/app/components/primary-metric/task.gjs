/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { assert } from '@ember/debug';
import { service } from '@ember/service';
import { task, timeout } from 'ember-concurrency';
import { didInsert, didUpdate } from '@ember/render-modifiers';
import { eq } from 'ember-truth-helpers';
import ENV from 'nomad-ui/config/environment';
import formatScheduledBytes from 'nomad-ui/helpers/format-scheduled-bytes';
import formatScheduledHertz from 'nomad-ui/helpers/format-scheduled-hertz';
import PrimaryMetricCurrentValue from 'nomad-ui/components/primary-metric/current-value';
import StatsTimeSeries from 'nomad-ui/components/stats-time-series';

export default class TaskPrimaryMetric extends Component {
  @service('stats-trackers-registry') statsTrackersRegistry;

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
    if (!this.tracker) return null;
    const task = this.tracker.tasks.findBy('task', this.taskState.name);
    if (this.metric === 'cpu') return task.reservedCPU;
    if (this.metric === 'memory') return task.reservedMemory;
    return null;
  }

  get chartClass() {
    if (this.metric === 'cpu') return 'is-info';
    if (this.metric === 'memory') return 'is-danger';
    return 'is-primary';
  }

  poller = task(async () => {
    do {
      this.tracker?.poll?.perform?.();
      await timeout(100);
    } while (ENV.environment !== 'test');
  });

  start = () => {
    this.taskState = this.args.taskState;
    this.tracker = this.statsTrackersRegistry.getTracker(
      this.args.taskState.allocation,
    );
    this.poller.perform();
  };

  willDestroy() {
    super.willDestroy(...arguments);
    this.poller.cancelAll();
    this.tracker?.signalPause?.perform?.();
  }

  <template>
    <div
      data-test-primary-metric
      class="primary-metric"
      ...attributes
      {{didInsert this.start}}
      {{didUpdate this.start @taskState @metric}}
    >
      <h4 data-test-primary-metric-title class="title is-5">
        {{#if (eq this.metric "cpu")}}
          CPU
        {{else if (eq this.metric "memory")}}
          Memory
        {{else}}
          {{this.metric}}
        {{/if}}
      </h4>
      <div class="primary-graphic">
        <StatsTimeSeries @data={{this.data}} @chartClass={{this.chartClass}} />
      </div>
      <PrimaryMetricCurrentValue
        @chartClass={{this.chartClass}}
        @percent={{this.data.lastObject.percent}}
      />
      <div class="annotation" data-test-absolute-value>
        {{#if (eq this.metric "cpu")}}
          <strong>{{formatScheduledHertz this.data.lastObject.used}}</strong>
          /
          {{formatScheduledHertz this.reservedAmount}}
          Total
        {{else if (eq this.metric "memory")}}
          <strong>{{formatScheduledBytes this.data.lastObject.used}}</strong>
          /
          {{formatScheduledBytes this.reservedAmount start="MiB"}}
          Total
        {{else}}
          <strong>{{this.data.lastObject.used}}</strong>
          /
          {{this.reservedAmount}}
          Total
        {{/if}}
      </div>
    </div>
  </template>
}
