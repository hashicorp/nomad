/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import hbs from 'htmlbars-inline-precompile';
import DelayedTruth from '../utils/delayed-truth';
import { withKnobs, optionsKnob, number } from '@storybook/addon-knobs';

export default {
  title: 'Charts/Recomendation CHart',
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
        />
      {{/if}}
    `,
    context: contextFactory(),
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
  };
}
