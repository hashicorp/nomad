/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { assert } from '@ember/debug';
import { service } from '@ember/service';
import { task, timeout } from 'ember-concurrency';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import didUpdate from '@ember/render-modifiers/modifiers/did-update';
import { eq } from 'ember-truth-helpers';
import ENV from 'nomad-ui/config/environment';
import formatScheduledBytes from 'nomad-ui/helpers/format-scheduled-bytes';
import formatScheduledHertz from 'nomad-ui/helpers/format-scheduled-hertz';
import { formatScheduledHertz as formatScheduledHertzValue } from 'nomad-ui/utils/units';
import PrimaryMetricCurrentValue from 'nomad-ui/components/primary-metric/current-value';
import StatsTimeSeries from 'nomad-ui/components/stats-time-series';

export default class NodePrimaryMetric extends Component {
  @service('stats-trackers-registry') statsTrackersRegistry;

  get metric() {
    assert('metric is a required argument', this.args.metric);
    return this.args.metric;
  }

  get tracker() {
    return this.statsTrackersRegistry.getTracker(this.args.node);
  }

  get data() {
    if (!this.tracker) return [];
    return this.tracker[this.metric];
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

  get reservedAnnotations() {
    const reserved = this.args.node?.reserved;

    if (this.metric === 'cpu' && reserved?.cpu) {
      const cpu = reserved.cpu;
      return [
        {
          label: `${formatScheduledHertzValue(cpu, 'MHz')} reserved`,
          percent: cpu / this.reservedAmount,
        },
      ];
    }

    if (this.metric === 'memory' && reserved?.memory) {
      const memory = reserved.memory;
      return [
        {
          label: `${formatScheduledBytes(memory, 'MiB')} reserved`,
          percent: memory / this.reservedAmount,
        },
      ];
    }

    return [];
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
        <StatsTimeSeries @data={{this.data}} @chartClass={{this.chartClass}}>
          <:after as |c|>
            {{#if this.reservedAnnotations}}
              <c.HAnnotations
                @annotations={{this.reservedAnnotations}}
                @labelProp="label"
              />
            {{/if}}
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
