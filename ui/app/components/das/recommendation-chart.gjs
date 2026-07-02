/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { next } from '@ember/runloop';
import { htmlSafe } from '@ember/template';
import { get } from '@ember/object';
import { fn, concat } from '@ember/helper';
import { on } from '@ember/modifier';
import { didInsert } from '@ember/render-modifiers';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import { eq } from 'ember-truth-helpers';
import windowResize from 'nomad-ui/modifiers/window-resize';

import { scaleLinear } from 'd3-scale';
import d3Format from 'd3-format';

const statsKeyToLabel = {
  min: 'Min',
  median: 'Median',
  mean: 'Mean',
  p99: '99th',
  max: 'Max',
  current: 'Current',
  recommended: 'New',
};

const formatPercent = d3Format.format('+.0%');

export default class RecommendationChart extends Component {
  @tracked width;
  @tracked height;

  @tracked shown = false;

  @tracked showLegend = false;
  @tracked mouseX;
  @tracked activeLegendRow;

  get isIncrease() {
    return this.args.currentValue < this.args.recommendedValue;
  }

  get directionClass() {
    if (this.args.disabled) {
      return 'disabled';
    } else if (this.isIncrease) {
      return 'increase';
    } else {
      return 'decrease';
    }
  }

  get icon() {
    return {
      x: 0,
      y: this.resourceLabel.y - this.iconHeight / 2,
      width: 20,
      height: this.iconHeight,
      name: this.isIncrease ? 'arrow-up' : 'arrow-down',
    };
  }

  gutterWidthLeft = 62;
  gutterWidthRight = 50;
  gutterWidth = this.gutterWidthLeft + this.gutterWidthRight;

  iconHeight = 21;

  tickTextHeight = 15;

  edgeTickHeight = 23;
  centerTickOffset = 6;

  centerY =
    this.tickTextHeight + this.centerTickOffset + this.edgeTickHeight / 2;

  edgeTickY1 = this.tickTextHeight + this.centerTickOffset;
  edgeTickY2 =
    this.tickTextHeight + this.edgeTickHeight + this.centerTickOffset;

  deltaTextY = this.edgeTickY2;

  meanHeight = this.edgeTickHeight * 0.6;
  p99Height = this.edgeTickHeight * 0.48;
  maxHeight = this.edgeTickHeight * 0.4;

  deltaTriangleHeight = this.edgeTickHeight / 2.5;

  get statsShapes() {
    if (this.width) {
      const maxShapes = this.shapesFor('max');
      const p99Shapes = this.shapesFor('p99');
      const meanShapes = this.shapesFor('mean');

      const labelProximityThreshold = 25;

      if (p99Shapes.text.x + labelProximityThreshold > maxShapes.text.x) {
        maxShapes.text.class = 'right';
      }

      if (meanShapes.text.x + labelProximityThreshold > p99Shapes.text.x) {
        p99Shapes.text.class = 'right';
      }

      if (meanShapes.text.x + labelProximityThreshold * 2 > maxShapes.text.x) {
        p99Shapes.text.class = 'hidden';
      }

      return [maxShapes, p99Shapes, meanShapes];
    } else {
      return [];
    }
  }

  shapesFor(key) {
    const stat = this.args.stats[key];

    const rectWidth = this.xScale(stat);
    const rectHeight = this[`${key}Height`];

    const tickX = rectWidth + this.gutterWidthLeft;

    const label = statsKeyToLabel[key];

    return {
      class: key,
      text: {
        label,
        x: tickX,
        y: this.tickTextHeight - 5,
        class: '',
      },
      line: {
        x1: tickX,
        y1: this.tickTextHeight,
        x2: tickX,
        y2: this.centerY - 2,
      },
      rect: {
        x: this.gutterWidthLeft,
        y:
          (this.edgeTickHeight - rectHeight) / 2 +
          this.centerTickOffset +
          this.tickTextHeight,
        width: rectWidth,
        height: rectHeight,
      },
    };
  }

  get barWidth() {
    return this.width - this.gutterWidth;
  }

  get higherValue() {
    return Math.max(this.args.currentValue, this.args.recommendedValue);
  }

  get maximumX() {
    return Math.max(
      this.higherValue,
      get(this.args.stats, 'max') || Number.MIN_SAFE_INTEGER,
    );
  }

  get lowerValue() {
    return Math.min(this.args.currentValue, this.args.recommendedValue);
  }

  get xScale() {
    return scaleLinear()
      .domain([0, this.maximumX])
      .rangeRound([0, this.barWidth]);
  }

  get lowerValueWidth() {
    return this.gutterWidthLeft + this.xScale(this.lowerValue);
  }

  get higherValueWidth() {
    return this.gutterWidthLeft + this.xScale(this.higherValue);
  }

  get center() {
    if (this.width) {
      return {
        x1: this.gutterWidthLeft,
        y1: this.centerY,
        x2: this.width - this.gutterWidthRight,
        y2: this.centerY,
      };
    } else {
      return null;
    }
  }

  get resourceLabel() {
    const text = this.args.resource === 'CPU' ? 'CPU' : 'Mem';

    return {
      text,
      x: this.gutterWidthLeft - 10,
      y: this.centerY,
    };
  }

  get deltaRect() {
    if (this.isIncrease) {
      return {
        x: this.lowerValueWidth,
        y: this.edgeTickY1,
        width: this.shown ? this.higherValueWidth - this.lowerValueWidth : 0,
        height: this.edgeTickHeight,
      };
    } else {
      return {
        x: this.shown ? this.lowerValueWidth : this.higherValueWidth,
        y: this.edgeTickY1,
        width: this.shown ? this.higherValueWidth - this.lowerValueWidth : 0,
        height: this.edgeTickHeight,
      };
    }
  }

  get deltaTriangle() {
    const directionXMultiplier = this.isIncrease ? 1 : -1;
    let translateX;

    if (this.shown) {
      translateX = this.isIncrease
        ? this.higherValueWidth
        : this.lowerValueWidth;
    } else {
      translateX = this.isIncrease
        ? this.lowerValueWidth
        : this.higherValueWidth;
    }

    return {
      style: htmlSafe(`transform: translateX(${translateX}px)`),
      points: `
        0,${this.center.y1}
        0,${this.center.y1 - this.deltaTriangleHeight / 2}
        ${(directionXMultiplier * this.deltaTriangleHeight) / 2},${this.center.y1}
        0,${this.center.y1 + this.deltaTriangleHeight / 2}
      `,
    };
  }

  get deltaLines() {
    if (this.isIncrease) {
      return {
        original: {
          x: this.lowerValueWidth,
        },
        delta: {
          style: htmlSafe(
            `transform: translateX(${this.shown ? this.higherValueWidth : this.lowerValueWidth}px)`,
          ),
        },
      };
    } else {
      return {
        original: {
          x: this.higherValueWidth,
        },
        delta: {
          style: htmlSafe(
            `transform: translateX(${this.shown ? this.lowerValueWidth : this.higherValueWidth}px)`,
          ),
        },
      };
    }
  }

  get deltaText() {
    const yOffset = 17;
    const y = this.deltaTextY + yOffset;

    const lowerValueText = {
      anchor: 'end',
      x: this.lowerValueWidth,
      y,
    };

    const higherValueText = {
      anchor: 'start',
      x: this.higherValueWidth,
      y,
    };

    const percentText = formatPercent(
      (this.args.recommendedValue - this.args.currentValue) /
        this.args.currentValue,
    );

    const percent = {
      x: (lowerValueText.x + higherValueText.x) / 2,
      y,
      text: percentText,
    };

    if (this.isIncrease) {
      return {
        original: lowerValueText,
        delta: higherValueText,
        percent,
      };
    } else {
      return {
        original: higherValueText,
        delta: lowerValueText,
        percent,
      };
    }
  }

  get chartHeight() {
    return this.deltaText.original.y + 1;
  }

  get tooltipStyle() {
    if (this.showLegend) {
      return htmlSafe(`left: ${this.mouseX}px`);
    }

    return undefined;
  }

  get sortedStats() {
    if (this.args.stats) {
      const statsWithCurrentAndRecommended = {
        ...this.args.stats,
        current: this.args.currentValue,
        recommended: this.args.recommendedValue,
      };

      return Object.keys(statsWithCurrentAndRecommended)
        .map((key) => ({
          label: statsKeyToLabel[key],
          value: statsWithCurrentAndRecommended[key],
        }))
        .sort((a, b) => a.value - b.value);
    } else {
      return [];
    }
  }

  isShown = () => {
    next(() => {
      this.shown = true;
    });
  };

  onResize = () => {
    this.width = this.svgElement.clientWidth;
    this.height = this.svgElement.clientHeight;
  };

  storeSvgElement = (element) => {
    this.svgElement = element;
  };

  setLegendPosition = (mouseMoveEvent) => {
    this.showLegend = true;
    this.mouseX = mouseMoveEvent.layerX;
  };

  hideLegend = () => {
    this.showLegend = false;
  };

  setActiveLegendRow = (row) => {
    this.activeLegendRow = row;
  };

  unsetActiveLegendRow = () => {
    this.activeLegendRow = undefined;
  };

  <template>
    <div
      ...attributes
      class={{concat "chart recommendation-chart " this.directionClass}}
      {{didInsert this.onResize}}
      {{windowResize this.onResize}}
      {{on "mousemove" this.setLegendPosition}}
      {{on "mouseleave" this.hideLegend}}
    >

      <svg
        class="chart"
        height={{this.chartHeight}}
        {{didInsert this.storeSvgElement}}
      >
        <svg
          class="icon delta"
          x={{this.icon.x}}
          y={{this.icon.y}}
          width={{this.icon.width}}
          height={{this.icon.height}}
        >
          <HdsIcon @name={{this.icon.name}} @isInline={{true}} />
        </svg>

        <text
          class="resource"
          x={{this.resourceLabel.x}}
          y={{this.resourceLabel.y}}
        >
          {{this.resourceLabel.text}}
        </text>

        {{#if this.center}}
          <line
            class="center"
            x1={{this.center.x1}}
            y1={{this.center.y1}}
            x2={{this.center.x2}}
            y2={{this.center.y2}}
          ></line>
        {{/if}}

        {{#each this.statsShapes as |shapes|}}
          <text
            class="stats-label {{shapes.text.class}}"
            text-anchor="end"
            x={{shapes.text.x}}
            y={{shapes.text.y}}
            data-test-label={{shapes.class}}
            {{on "mouseenter" (fn this.setActiveLegendRow shapes.text.label)}}
            {{on "mouseleave" this.unsetActiveLegendRow}}
          >
            {{shapes.text.label}}
          </text>

          <rect
            class="stat {{shapes.class}}"
            x={{shapes.rect.x}}
            width={{shapes.rect.width}}
            y={{shapes.rect.y}}
            height={{shapes.rect.height}}
            {{on "mouseenter" (fn this.setActiveLegendRow shapes.text.label)}}
            {{on "mouseleave" this.unsetActiveLegendRow}}
          ></rect>

          <line
            class="stat {{shapes.class}}"
            x1={{shapes.line.x1}}
            y1={{shapes.line.y1}}
            x2={{shapes.line.x2}}
            y2={{shapes.line.y2}}
            {{on "mouseenter" (fn this.setActiveLegendRow shapes.text.label)}}
            {{on "mouseleave" this.unsetActiveLegendRow}}
          ></line>
        {{/each}}

        {{#unless @disabled}}
          {{#if this.deltaRect.x}}
            <rect
              {{didInsert this.isShown}}
              class="delta"
              x={{this.deltaRect.x}}
              y={{this.deltaRect.y}}
              width={{this.deltaRect.width}}
              height={{this.deltaRect.height}}
            ></rect>

            <polygon
              class="delta"
              style={{this.deltaTriangle.style}}
              points={{this.deltaTriangle.points}}
            ></polygon>

            <line
              class="changes delta"
              style={{this.deltaLines.delta.style}}
              x1={{0}}
              y1={{this.edgeTickY1}}
              x2={{0}}
              y2={{this.edgeTickY2}}
              {{on "mouseenter" (fn this.setActiveLegendRow "New")}}
              {{on "mouseleave" this.unsetActiveLegendRow}}
            ></line>

            <line
              class="changes"
              x1={{this.deltaLines.original.x}}
              y1={{this.edgeTickY1}}
              x2={{this.deltaLines.original.x}}
              y2={{this.edgeTickY2}}
              {{on "mouseenter" (fn this.setActiveLegendRow "Current")}}
              {{on "mouseleave" this.unsetActiveLegendRow}}
            ></line>

            <text
              class="changes"
              text-anchor={{this.deltaText.original.anchor}}
              x={{this.deltaText.original.x}}
              y={{this.deltaText.original.y}}
              {{on "mouseenter" (fn this.setActiveLegendRow "Current")}}
              {{on "mouseleave" this.unsetActiveLegendRow}}
            >
              Current
            </text>

            <text
              class="changes new"
              text-anchor={{this.deltaText.delta.anchor}}
              x={{this.deltaText.delta.x}}
              y={{this.deltaText.delta.y}}
              {{on "mouseenter" (fn this.setActiveLegendRow "New")}}
              {{on "mouseleave" this.unsetActiveLegendRow}}
            >
              New
            </text>

            <text
              class="changes percent"
              x={{this.deltaText.percent.x}}
              y={{this.deltaText.percent.y}}
            >
              {{this.deltaText.percent.text}}
            </text>
          {{/if}}
        {{/unless}}

        <line
          class="zero"
          x1={{this.gutterWidthLeft}}
          y1={{this.edgeTickY1}}
          x2={{this.gutterWidthLeft}}
          y2={{this.edgeTickY2}}
        ></line>
      </svg>

      <div
        class="chart-tooltip {{if this.showLegend 'active' 'inactive'}}"
        style={{this.tooltipStyle}}
      >
        <ol>
          {{#each this.sortedStats as |stat|}}
            <li class={{if (eq this.activeLegendRow stat.label) "active"}}>
              <span class="label">
                {{stat.label}}
              </span>
              <span class="value">{{stat.value}}</span>
            </li>
          {{/each}}
        </ol>
      </div>

    </div>
  </template>
}
