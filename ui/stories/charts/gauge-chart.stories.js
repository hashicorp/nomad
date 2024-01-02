/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';
import DelayedArray from '../utils/delayed-array';
import DelayedTruth from '../utils/delayed-truth';

export default {
  title: 'Charts/Gauge Chart',
};

let totalVariations = [
  { value: 0, total: 10 },
  { value: 1, total: 10 },
  { value: 2, total: 10 },
  { value: 3, total: 10 },
  { value: 4, total: 10 },
  { value: 5, total: 10 },
  { value: 6, total: 10 },
  { value: 7, total: 10 },
  { value: 8, total: 10 },
  { value: 9, total: 10 },
  { value: 10, total: 10 },
];

let complementVariations = [
  { value: 0, complement: 10 },
  { value: 1, complement: 9 },
  { value: 2, complement: 8 },
  { value: 3, complement: 7 },
  { value: 4, complement: 6 },
  { value: 5, complement: 5 },
  { value: 6, complement: 4 },
  { value: 7, complement: 3 },
  { value: 8, complement: 2 },
  { value: 9, complement: 1 },
  { value: 10, complement: 0 },
];

let colorVariations = ['is-info', 'is-warning', 'is-success', 'is-danger'];

export let Total = () => {
  return {
    template: hbs`
      <div class="multiples">
        {{#each variations as |v|}}
          <div class="chart-container">
            <GaugeChart @value={{v.value}} @total={{v.total}} @label="Total" @chartClass="is-info" />
          </div>
        {{/each}}
      </div>
    `,
    context: {
      variations: DelayedArray.create(totalVariations),
    },
  };
};

export let Complement = () => {
  return {
    template: hbs`
      <div class="multiples">
        {{#each variations as |v|}}
          <div class="chart-container">
            <GaugeChart @value={{v.value}} @complement={{v.complement}} @label="Complement" @chartClass="is-info" />
          </div>
        {{/each}}
      </div>
    `,
    context: {
      variations: DelayedArray.create(complementVariations),
    },
  };
};

export let Colors = () => {
  return {
    template: hbs`
      <div class="multiples">
        {{#each variations as |color|}}
          <div class="chart-container">
            <GaugeChart @value={{7}} @total={{10}} @label={{color}} @chartClass={{color}} />
          </div>
        {{/each}}
      </div>
    `,
    context: {
      variations: DelayedArray.create(colorVariations),
    },
  };
};

export let Sizing = () => {
  return {
    template: hbs`
      {{#if delayedTruth.complete}}
        <div class="multiples">
          <div class="chart-container is-small">
            <GaugeChart @value={{7}} @total={{10}} @label="Small" />
          </div>
          <div class="chart-container">
            <GaugeChart @value={{7}} @total={{10}} @label="Regular" />
          </div>
          <div class="chart-container is-large">
            <GaugeChart @value={{7}} @total={{10}} @label="Large" />
          </div>
          <div class="chart-container is-xlarge">
            <GaugeChart @value={{7}} @total={{10}} @label="X-Large" />
          </div>
        </div>
      {{/if}}
      <p class="annotation">GaugeCharts fill the width of their container and have a dynamic height according to the height of the arc. However, the text within a gauge chart is fixed. This can create unsightly overlap or whitespace, so be careful about responsiveness when using this chart type.</p>
    `,
    context: {
      delayedTruth: DelayedTruth.create(),
    },
  };
};
