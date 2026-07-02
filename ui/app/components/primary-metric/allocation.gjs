/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { assert } from '@ember/debug';
import { service } from '@ember/service';
import { task, timeout } from 'ember-concurrency';
import { didInsert, didUpdate } from '@ember/render-modifiers';
import { eq } from 'ember-truth-helpers';
import ENV from 'nomad-ui/config/environment';
import formatScheduledBytes from 'nomad-ui/helpers/format-scheduled-bytes';
import formatScheduledHertz from 'nomad-ui/helpers/format-scheduled-hertz';
import reverse from '@nullvoxpopuli/ember-composable-helpers/helpers/reverse';
import PrimaryMetricCurrentValue from 'nomad-ui/components/primary-metric/current-value';
import StatsTimeSeries from 'nomad-ui/components/stats-time-series';

export default class AllocationPrimaryMetric extends Component {
  @service('stats-trackers-registry') statsTrackersRegistry;

  get metric() {
    assert('metric is a required argument', this.args.metric);
    return this.args.metric;
  }

  get allocation() {
    return this.args.allocation;
  }

  get tracker() {
    return this.statsTrackersRegistry.getTracker(this.allocation);
  }

  get data() {
    if (!this.tracker) return [];
    return this.tracker[this.metric];
  }

  get series() {
    if (!this.tracker?.tasks) {
      return [];
    }

    return this.tracker.tasks
      .map((task) => ({
        name: task.task,
        data: task[this.metric],
      }))
      .reverse();
  }

  get reservedAmount() {
    if (!this.tracker) return null;
    if (this.metric === 'cpu') return this.tracker.reservedCPU;
    if (this.metric === 'memory') return this.tracker.reservedMemory;
    return null;
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

  poller = task(async () => {
    do {
      this.tracker?.poll?.perform?.();
      await timeout(100);
    } while (ENV.environment !== 'test');
  });

  start = () => {
    if (this.tracker) this.poller.perform();
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
      {{didUpdate this.start}}
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
        <StatsTimeSeries @data={{this.series}} @dataProp="data">
          <:svg as |c|>
            {{#each (reverse this.series) as |series idx|}}
              <c.Area
                @data={{series.data}}
                @colorScale={{this.colorScale}}
                @index={{idx}}
                data-test-task-name={{series.name}}
              />
            {{/each}}
          </:svg>
          <:after as |c|>
            <c.Tooltip class="is-snappy" as |series datum idx|>
              <li>
                <span class="label"><span
                    class="color-swatch swatch-{{this.colorScale}}
                      swatch-{{this.colorScale}}-{{idx}}"
                  />{{series.name}}</span>
                {{#if (eq this.metric "cpu")}}
                  <span class="value">{{formatScheduledHertz
                      datum.datum.used
                    }}</span>
                {{else if (eq this.metric "memory")}}
                  <span class="value">{{formatScheduledBytes
                      datum.datum.used
                    }}</span>
                {{else}}
                  <span class="value">{{datum.formatttedY}}</span>
                {{/if}}
              </li>
            </c.Tooltip>
          </:after>
        </StatsTimeSeries>
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
