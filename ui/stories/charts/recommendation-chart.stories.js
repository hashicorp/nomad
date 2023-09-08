/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';
import DelayedTruth from '../utils/delayed-truth';
import {
  withKnobs,
  optionsKnob,
  number,
  boolean,
} from '@storybook/addon-knobs';

export default {
  title: 'Charts/Recomendation Chart',
  decorators: [withKnobs],
};

export let Configurable = () => {
  return {
    template: hbs`
      <SvgPatterns />
      {{#if delayedTruth.complete}}
        <Das::RecommendationChart
          @resource={{resource}}
          @currentValue={{current}}
          @recommendedValue={{recommendedValue}}
          @stats={{stats}}
          @disabled={{disabled}}
        />
      {{/if}}
    `,
    context: contextFactory(),
  };
};

export let Standard = () => {
  return {
    template: hbs`
      <SvgPatterns />
      <div style="max-width: 500px">
        {{#if delayedTruth.complete}}
          <Das::RecommendationChart
            @resource="CPU"
            @currentValue={{cpu.current}}
            @recommendedValue={{cpu.recommendedValue}}
            @stats={{cpu.stats}}
          />
          <Das::RecommendationChart
            @resource="MemoryMB"
            @currentValue={{mem.current}}
            @recommendedValue={{mem.recommendedValue}}
            @stats={{mem.stats}}
          />
          <hr/>
          <Das::RecommendationChart
            @resource="CPU"
            @currentValue={{cpu.current}}
            @recommendedValue={{cpu.recommendedValue}}
            @stats={{cpu.stats}}
            @disabled={{true}}
          />
          <Das::RecommendationChart
            @resource="MemoryMB"
            @currentValue={{mem.current}}
            @recommendedValue={{mem.recommendedValue}}
            @stats={{mem.stats}}
          />
        {{/if}}
      </div>
    `,
    context: {
      delayedTruth: DelayedTruth.create(),
      cpu: {
        current: 100,
        recommendedValue: 600,
        stats: {
          mean: 300,
          p99: 500,
          max: 525,
        },
      },
      mem: {
        current: 2048,
        recommendedValue: 256,
        stats: {
          mean: 140,
          p99: 215,
          max: 225,
        },
      },
    },
  };
};

function contextFactory() {
  const numberConfig = { range: true, min: 0, max: 1000, step: 1 };
  return {
    delayedTruth: DelayedTruth.create(),
    resource: optionsKnob(
      'Resource',
      { Cpu: 'CPU', Memory: 'MemoryMB' },
      'CPU',
      { display: 'inline-radio' }
    ),
    current: number('Current', 100, numberConfig),
    recommendedValue: number('Recommendation', 300, numberConfig),
    stats: {
      mean: number('Stat: mean', 150, numberConfig),
      p99: number('Stat: p99', 600, numberConfig),
      max: number('Stat: max', 650, numberConfig),
    },
    disabled: boolean('Disabled', false),
  };
}
