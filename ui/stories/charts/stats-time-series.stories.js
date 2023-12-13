/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

import EmberObject, { computed } from '@ember/object';
import { on } from '@ember/object/evented';
import moment from 'moment';

import DelayedArray from '../utils/delayed-array';

export default {
  title: 'Charts/Stats Time Series',
};

let ts = (offset) => moment().subtract(offset, 'm').toDate();

export let Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Stats Time Series</h5>
      <div class="block" style="height:100px; width: 400px;">
        {{#if staticMetrics}}
          <StatsTimeSeries @data={{staticMetrics}} @chartClass="is-primary" />
        {{/if}}
      </div>
      `,
    context: {
      staticMetrics: DelayedArray.create([
        { timestamp: ts(20), percent: 0.5 },
        { timestamp: ts(18), percent: 0.5 },
        { timestamp: ts(16), percent: 0.4 },
        { timestamp: ts(14), percent: 0.3 },
        { timestamp: ts(12), percent: 0.9 },
        { timestamp: ts(10), percent: 0.3 },
        { timestamp: ts(8), percent: 0.3 },
        { timestamp: ts(6), percent: 0.4 },
        { timestamp: ts(4), percent: 0.5 },
        { timestamp: ts(2), percent: 0.6 },
        { timestamp: ts(0), percent: 0.6 },
      ]),
    },
  };
};

export let HighLowComparison = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Stats Time Series high/low comparison</h5>
      <div class="columns">
        <div class="block column" style="height:200px; width:400px">
          {{#if data.metricsHigh}}
            <StatsTimeSeries @data={{data.metricsHigh}} @chartClass="is-info" />
          {{/if}}
        </div>
        <div class="block column" style="height:200px; width:400px">
          {{#if data.metricsLow}}
            <StatsTimeSeries @data={{data.metricsLow}} @chartClass="is-info" />
          {{/if}}
        </div>
      </div>
      <p class="annotation">Line charts, and therefore stats time series charts, use a letant linear gradient with a height equal to the canvas. This makes the color intensity of the gradient at values consistent across charts as long as those charts have the same y-axis domain.</p>
      <p class="annotation">This is used to great effect with stats charts since they all have a y-axis domain of 0-100%.</p>
      `,
    context: {
      data: EmberObject.extend({
        timerTicks: 0,

        startTimer: on('init', function () {
          this.set(
            'timer',
            setInterval(() => {
              let metricsHigh = this.metricsHigh;
              let prev = metricsHigh.length
                ? metricsHigh[metricsHigh.length - 1].percent
                : 0.9;
              this.appendTSValue(
                metricsHigh,
                Math.min(Math.max(prev + Math.random() * 0.05 - 0.025, 0.5), 1)
              );

              let metricsLow = this.metricsLow;
              let prev2 = metricsLow.length
                ? metricsLow[metricsLow.length - 1].percent
                : 0.1;
              this.appendTSValue(
                metricsLow,
                Math.min(Math.max(prev2 + Math.random() * 0.05 - 0.025, 0), 0.5)
              );
            }, 1000)
          );
        }),

        appendTSValue(array, percent, maxLength = 300) {
          array.addObject({
            timestamp: Date.now(),
            percent,
          });

          if (array.length > maxLength) {
            array.splice(0, array.length - maxLength);
          }
        },

        willDestroy() {
          clearInterval(this.timer);
        },

        metricsHigh: computed(function () {
          return [];
        }),

        metricsLow: computed(function () {
          return [];
        }),

        secondsFormat() {
          return (date) => moment(date).format('HH:mm:ss');
        },
      }).create(),
    },
  };
};
