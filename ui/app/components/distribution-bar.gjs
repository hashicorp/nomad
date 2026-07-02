/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { run, once, schedule } from '@ember/runloop';
import { guidFor } from '@ember/object/internals';
import { concat, hash } from '@ember/helper';
import { eq } from 'ember-truth-helpers';
import { didInsert, didUpdate } from '@ember/render-modifiers';
import { copy } from 'ember-copy';
import d3 from 'd3-selection';
import 'd3-transition';
import windowResize from 'nomad-ui/modifiers/window-resize';

const sumAggregate = (total, val) => total + val;

export default class DistributionBar extends Component {
  @tracked activeDatum = null;
  @tracked isActive = false;
  @tracked tooltipPosition = null;

  chart = null;
  slices = null;
  maskId = null;
  rootElement = null;
  svgElement = null;

  get data() {
    return this.args.data ?? [];
  }

  get onSliceClick() {
    return this.args.onSliceClick;
  }

  get isNarrow() {
    return this.args.isNarrow ?? false;
  }

  get _data() {
    const data = copy(this.data, true);
    const sum = data.mapBy('value').reduce(sumAggregate, 0);

    return data.map(
      ({ label, value, className, layers, legendLink, help }, index) => ({
        label,
        value,
        className,
        layers,
        legendLink,
        help,
        index,
        percent: value / sum,
        offset:
          data.slice(0, index).mapBy('value').reduce(sumAggregate, 0) / sum,
      }),
    );
  }

  get tooltipStyle() {
    if (!this.tooltipPosition) {
      return '';
    }

    return Object.keys(this.tooltipPosition)
      .map((key) => {
        const value = this.tooltipPosition[key];
        const formatted =
          typeof value === 'number' ? `${value.toFixed(2)}px` : value;
        return `${key}:${formatted}`;
      })
      .join(';');
  }

  setupChart = (svgElement) => {
    this.svgElement = svgElement;
    this.rootElement = svgElement.closest('.distribution-bar');
    this.chart = d3.select(svgElement);
    this.maskId = `dist-mask-${guidFor(this)}`;

    svgElement.querySelector('clipPath').setAttribute('id', this.maskId);

    this.chart.on('mouseleave', () => {
      run(() => {
        this.isActive = false;
        this.activeDatum = null;
        this.chart
          .selectAll('g')
          .classed('active', false)
          .classed('inactive', false);
      });
    });

    this.renderChart();
  };

  renderChart = () => {
    const { chart, _data, isNarrow, svgElement } = this;

    if (!chart || !svgElement) {
      return;
    }

    const width = svgElement.clientWidth;
    const filteredData = _data.filter((d) => d.value > 0);
    filteredData.forEach((d, index) => {
      d.index = index;
    });

    let slices = chart
      .select('.bars')
      .selectAll('g')
      .data(filteredData, (d) => d.label);
    const sliceCount = filteredData.length;

    slices.exit().remove();

    const slicesEnter = slices
      .enter()
      .append('g')
      .on('mouseenter', (ev, d) => {
        run(() => {
          const allSlices = this.slices;
          const slice = allSlices.filter((datum) => datum.label === d.label);
          allSlices.classed('active', false).classed('inactive', true);
          slice.classed('active', true).classed('inactive', false);
          this.activeDatum = d;

          const box = slice.node().getBBox();
          const pos = box.x + box.width / 2;

          // Ensure that the position is set before the tooltip is visible.
          schedule('afterRender', this, () => {
            this.isActive = true;
          });
          this.tooltipPosition = { left: pos };
        });
      });

    slices = slices.merge(slicesEnter);
    slices
      .attr('class', (d) => {
        const className = d.className || `slice-${_data.indexOf(d)}`;
        const activeDatum = this.activeDatum;
        const isActive = activeDatum && activeDatum.label === d.label;
        const isInactive = activeDatum && activeDatum.label !== d.label;
        const isClickable = !!this.onSliceClick;
        return [
          className,
          isActive && 'active',
          isInactive && 'inactive',
          isClickable && 'clickable',
        ]
          .filter(Boolean)
          .join(' ');
      })
      .attr('data-test-slice-label', (d) => d.className);

    this.slices = slices;

    const setWidth = (d) => {
      // Remove a pixel from either side of the slice.
      let modifier = 2;
      if (d.index === 0) modifier--;
      if (d.index === sliceCount - 1) modifier--;

      return `${width * d.percent - modifier}px`;
    };

    const setOffset = (d) => `${width * d.offset + (d.index === 0 ? 0 : 1)}px`;

    let hoverTargets = slices.selectAll('.target').data((d) => [d]);
    hoverTargets
      .enter()
      .append('rect')
      .attr('class', 'target')
      .attr('width', setWidth)
      .attr('height', '100%')
      .attr('x', setOffset)
      .merge(hoverTargets)
      .transition()
      .duration(200)
      .attr('width', setWidth)
      .attr('x', setOffset);

    let layers = slices.selectAll('.bar').data((d, i) => {
      return new Array(d.layers || 1).fill(Object.assign({ index: i }, d));
    });

    layers
      .enter()
      .append('rect')
      .attr('width', setWidth)
      .attr('x', setOffset)
      .attr('y', () => (isNarrow ? '50%' : 0))
      .attr('clip-path', `url(#${this.maskId})`)
      .attr('height', () => (isNarrow ? '6px' : '100%'))
      .attr('transform', () => (isNarrow ? 'translate(0, -3)' : ''))
      .merge(layers)
      .attr('class', (d, i) => `bar layer-${i}`)
      .transition()
      .duration(200)
      .attr('width', setWidth)
      .attr('x', setOffset);

    if (isNarrow && this.rootElement) {
      d3.select(this.rootElement)
        .select('.mask')
        .attr('height', '6px')
        .attr('y', '50%');
    }

    if (this.onSliceClick) {
      slices.on('click', this.onSliceClick);
    }
  };

  windowResizeHandler = () => {
    once(this, this.renderChart);
  };

  <template>
    <div
      class="chart distribution-bar {{if this.isNarrow 'is-narrow'}}"
      ...attributes
      {{windowResize this.windowResizeHandler}}
    >
      <svg
        {{didInsert this.setupChart}}
        {{didUpdate this.renderChart this._data this.isNarrow}}
      >
        <defs>
          <clipPath>
            <rect
              class="mask"
              x="0"
              y="0"
              width="100%"
              height="100%"
              rx="2px"
              ry="2px"
            ></rect>
          </clipPath>
        </defs>
        <g class="bars"></g>
      </svg>
      {{#if (has-block)}}
        {{yield (hash data=this._data activeDatum=this.activeDatum)}}
      {{else}}
        <div
          class="chart-tooltip with-active-datum
            {{if this.isActive 'active' 'inactive'}}"
          style={{this.tooltipStyle}}
        >
          <ol>
            {{#each this._data as |datum index|}}
              <li
                class="{{if
                    (eq datum.label this.activeDatum.label)
                    'is-active'
                  }}"
              >
                <span class="label {{if (eq datum.value 0) 'is-empty'}}">
                  <span
                    class="color-swatch
                      {{if
                        datum.className
                        datum.className
                        (concat 'swatch-' index)
                      }}"
                  ></span>
                  {{datum.label}}
                </span>
                <span class="value">{{datum.value}}</span>
              </li>
            {{/each}}
          </ol>
        </div>
      {{/if}}
    </div>
  </template>
}
