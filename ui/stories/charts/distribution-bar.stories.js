/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

import EmberObject, { computed } from '@ember/object';
import { on } from '@ember/object/evented';

import DelayedTruth from '../utils/delayed-truth';

export default {
  title: 'Charts/Distribution Bar',
};

export let Standard = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Distribution Bar</h5>
      <div class="block" style="height:50px; width:200px;">
        {{#if delayedTruth.complete}}
          <DistributionBar @data={{distributionBarData}} />
        {{/if}}
      </div>
      <p class="annotation">The distribution bar chart proportionally show data in a single bar. It includes a tooltip out of the box, assumes the size of the container element, and is designed to be styled with CSS.</p>
      `,
    context: {
      delayedTruth: DelayedTruth.create(),
      distributionBarData: [
        { label: 'one', value: 10 },
        { label: 'two', value: 20 },
        { label: 'three', value: 30 },
      ],
    },
  };
};

export let WithClasses = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Distribution Bar with classes</h5>
      <div class="block" style="height:50px; width:200px;">
        {{#if delayedTruth.complete}}
          <DistributionBar @data={{distributionBarDataWithClasses}} />
        {{/if}}
      </div>
      <p class="annotation">If a datum provides a <code>className</code> property, it will be assigned to the corresponding <code>rect</code> element, allowing for custom colorization.</p>
      `,
    context: {
      delayedTruth: DelayedTruth.create(),
      distributionBarDataWithClasses: [
        { label: 'Queued', value: 10, className: 'queued' },
        { label: 'Complete', value: 20, className: 'complete' },
        { label: 'Failed', value: 30, className: 'failed' },
      ],
    },
  };
};

export let Flexibility = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Distribution Bar flexibility</h5>
      <div class="block" style="height:10px; width:600px;">
        {{#if delayedTruth.complete}}
          <DistributionBar @data={{distributionBarData}} />
        {{/if}}
      </div>
      <div class="block" style="height:200px; width:30px;">
        {{#if delayedTruth.complete}}
          <DistributionBar @data={{distributionBarData}} />
        {{/if}}
      </div>
      <p class="annotation">Distribution bar assumes the dimensions of the container.</p>
      `,
    context: {
      delayedTruth: DelayedTruth.create(),
      distributionBarData: [
        { label: 'one', value: 10 },
        { label: 'two', value: 20 },
        { label: 'three', value: 30 },
      ],
    },
  };
};

export let LiveUpdating = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Live-updating Distribution Bar</h5>
      <div class="block" style="height:50px; width:600px;">
        <DistributionBar @data={{controller.distributionBarDataRotating}} />
      </div>
      <p class="annotation">Distribution bar animates with data changes.</p>
      <div class="boxed-section">
        <div class="boxed-section-body is-dark">
          <JsonViewer @json={{controller.distributionBarDataRotating}} />
        </div>
      </div>
      `,
    context: {
      controller: EmberObject.extend({
        timerTicks: 0,

        startTimer: on('init', function () {
          this.set(
            'timer',
            setInterval(() => {
              this.incrementProperty('timerTicks');
            }, 500)
          );
        }),

        willDestroy() {
          clearInterval(this.timer);
        },

        distributionBarDataRotating: computed('timerTicks', function () {
          return [
            { label: 'one', value: Math.round(Math.random() * 50) },
            { label: 'two', value: Math.round(Math.random() * 50) },
            { label: 'three', value: Math.round(Math.random() * 50) },
          ];
        }),
      }).create(),
    },
  };
};

export let SingleBar = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Distribution Bar with single bar</h5>
      <div class="block" style="height:50px; width:600px;">
        {{#if delayedTruth.complete}}
          <DistributionBar @data={{distributionBarDatum}} />
        {{/if}}
      </div>
      `,
    context: {
      delayedTruth: DelayedTruth.create(),
      distributionBarDatum: [{ label: 'one', value: 10 }],
    },
  };
};

export let Jumbo = () => {
  return {
    template: hbs`
      <h5 class="title is-5">Jumbo Distribution Bar</h5>
      {{#if delayedTruth.complete}}
        <DistributionBar @data={{distributionBarData}} @class="split-view" as |chart|>
          <ol class="legend">
            {{#each chart.data as |datum index|}}
              <li class="{{datum.className}} {{if (eq datum.index chart.activeDatum.index) "is-active"}} {{if (eq datum.value 0) "is-empty"}}">
                <span class="color-swatch {{if datum.className datum.className (concat "swatch-" index)}}" />
                <span class="value" data-test-legend-value="{{datum.className}}">{{datum.value}}</span>
                <span class="label">
                  {{datum.label}}
                </span>
              </li>
            {{/each}}
          </ol>
        </DistributionBar>
      {{/if}}
      <p class="annotation">A variation of the Distribution Bar component for when the distribution bar is the central component of the page. It's a larger format that requires no interaction to see the data labels and values.</p>
      `,
    context: {
      delayedTruth: DelayedTruth.create(),
      distributionBarData: [
        { label: 'one', value: 10 },
        { label: 'two', value: 20 },
        { label: 'three', value: 0 },
        { label: 'four', value: 35 },
      ],
    },
  };
};
