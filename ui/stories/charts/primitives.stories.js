/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import hbs from 'htmlbars-inline-precompile';

export default {
  title: 'Charts/Primitives',
};

export let Tooltip = () => ({
  template: hbs`
    <h5 class="title is-5">Single Entry</h5>
    <div class="mock-hover-region" style="width:300px;height:100px">
      <ChartPrimitives::Tooltip @active={{true}} @style={{this.style}} @data={{this.dataSingle}} as |series|>
        <li>
          <span class="label"><span class="color-swatch swatch-reds" />{{series.name}}</span>
          <span class="value">{{series.value}}</span>
        </li>
      </ChartPrimitives::Tooltip>
    </div>

    <h5 class="title is-5">Multiple Entries</h5>
    <div class="mock-hover-region" style="width:300px;height:100px">
      <ChartPrimitives::Tooltip @active={{true}} @style={{this.style}} @data={{take 4 this.dataMultiple}} as |series datum index|>
        <li>
          <span class="label"><span class="color-swatch swatch-reds swatch-reds-{{index}}" />{{series.name}}</span>
          <span class="value">{{datum.value}}</span>
        </li>
      </ChartPrimitives::Tooltip>
    </div>

    <h5 class="title is-5">Active Entry</h5>
    <div class="mock-hover-region" style="width:300px;height:100px">
      <ChartPrimitives::Tooltip @active={{true}} @style={{this.style}} @data={{take 4 this.dataMultiple}} class="with-active-datum" as |series datum index|>
        <li class="{{if (eq series.name "Three") "is-active"}}">
          <span class="label"><span class="color-swatch swatch-reds swatch-reds-{{index}}" />{{series.name}}</span>
          <span class="value">{{datum.value}}</span>
        </li>
      </ChartPrimitives::Tooltip>
    </div>

    <h5 class="title is-5">Color Scales</h5>
    <div class="multiples is-left-aligned with-spacing">
      {{#each this.scales as |scale|}}
        <div class="mock-hover-region" style="width:300px;height:200px">
          {{scale}}
          <ChartPrimitives::Tooltip @active={{true}} @style="left:70%;top:75%" @data={{this.dataMultiple}} as |series datum index|>
            <li>
              <span class="label"><span class="color-swatch swatch-{{scale}} swatch-{{scale}}-{{index}}" />{{series.name}}</span>
              <span class="value">{{datum.value}}</span>
            </li>
          </ChartPrimitives::Tooltip>
        </div>
      {{/each}}
    </div>
  `,
  context: {
    style: 'left:70%;top:50%;',
    dataSingle: [{ series: { name: 'Example', value: 12 } }],
    dataMultiple: [
      { series: { name: 'One' }, datum: { value: 12 }, index: 0 },
      { series: { name: 'Two' }, datum: { value: 24 }, index: 1 },
      { series: { name: 'Three' }, datum: { value: 36 }, index: 2 },
      { series: { name: 'Four' }, datum: { value: 48 }, index: 3 },
      { series: { name: 'Five' }, datum: { value: 60 }, index: 4 },
      { series: { name: 'Six' }, datum: { value: 72 }, index: 5 },
      { series: { name: 'Seven' }, datum: { value: 84 }, index: 6 },
    ],
    scales: ['reds', 'blues', 'ordinal'],
  },
});
