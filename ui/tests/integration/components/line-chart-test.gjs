/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  click,
  find,
  findAll,
  render,
  triggerEvent,
} from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import sinon from 'sinon';
import moment from 'moment';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import LineChart from 'nomad-ui/components/line-chart';

const REF_DATE = new Date();

module('Integration | Component | line-chart', function (hooks) {
  setupRenderingTest(hooks);

  test('when a chart has annotations, they are rendered in order', async function (assert) {
    const annotations = [
      { x: 2, type: 'info' },
      { x: 1, type: 'error' },
      { x: 3, type: 'info' },
    ];
    this.setProperties({
      annotations,
      data: [
        { x: 1, y: 1 },
        { x: 10, y: 10 },
      ],
    });

    await render(
      <template>
        <LineChart @xProp="x" @yProp="y" @data={{this.data}}>
          <:after as |c|>
            <c.VAnnotations @annotations={{this.annotations}} />
          </:after>
        </LineChart>
      </template>,
    );

    const sortedAnnotations = [...annotations].sort((a, b) => (a.x || 0) - (b.x || 0));
    findAll('[data-test-annotation]').forEach((annotation, index) => {
      const datum = sortedAnnotations[index];
      assert.deepEqual(
        annotation.querySelector('button').getAttribute('title'),
        `${datum.type} event at ${datum.x}`,
      );
    });

    await componentA11yAudit(this.element, assert);
  });

  test('when a chart has annotations and is timeseries, annotations are sorted reverse-chronologically', async function (assert) {
    const annotations = [
      {
        x: moment(REF_DATE).add(2, 'd').toDate(),
        type: 'info',
      },
      {
        x: moment(REF_DATE).add(1, 'd').toDate(),
        type: 'error',
      },
      {
        x: moment(REF_DATE).add(3, 'd').toDate(),
        type: 'info',
      },
    ];
    this.setProperties({
      annotations,
      data: [
        { x: 1, y: 1 },
        { x: 10, y: 10 },
      ],
    });

    await render(
      <template>
        <LineChart
          @xProp="x"
          @yProp="y"
          @timeseries={{true}}
          @data={{this.data}}
        >
          <:after as |c|>
            <c.VAnnotations @annotations={{this.annotations}} />
          </:after>
        </LineChart>
      </template>,
    );

    const sortedAnnotations = [...annotations].sort((a, b) => (b.x || 0) - (a.x || 0));
    findAll('[data-test-annotation]').forEach((annotation, index) => {
      const datum = sortedAnnotations[index];
      assert.deepEqual(
        annotation.querySelector('button').getAttribute('title'),
        `${datum.type} event at ${moment(datum.x).format('MMM DD, HH:mm')}`,
      );
    });
  });

  test('clicking annotations calls the onAnnotationClick action with the annotation as an argument', async function (assert) {
    const annotations = [{ x: 2, type: 'info', meta: { data: 'here' } }];
    this.setProperties({
      annotations,
      data: [
        { x: 1, y: 1 },
        { x: 10, y: 10 },
      ],
      click: sinon.spy(),
    });

    await render(
      <template>
        <LineChart @xProp="x" @yProp="y" @data={{this.data}}>
          <:after as |c|>
            <c.VAnnotations
              @annotations={{this.annotations}}
              @annotationClick={{this.click}}
            />
          </:after>
        </LineChart>
      </template>,
    );

    await click('[data-test-annotation] button');
    assert.ok(this.click.calledWith(annotations[0]));
  });

  test('annotations will have staggered heights when too close to be positioned side-by-side', async function (assert) {
    const annotations = [
      { x: 2, type: 'info' },
      { x: 2.4, type: 'error' },
      { x: 9, type: 'info' },
    ];
    this.setProperties({
      annotations,
      data: [
        { x: 1, y: 1 },
        { x: 10, y: 10 },
      ],
      click: sinon.spy(),
    });

    await render(
      <template>
        <div style="width:200px;">
          <LineChart @xProp="x" @yProp="y" @data={{this.data}}>
            <:after as |c|>
              <c.VAnnotations @annotations={{this.annotations}} />
            </:after>
          </LineChart>
        </div>
      </template>,
    );

    const annotationElements = findAll('[data-test-annotation]');
    assert.notOk(annotationElements[0].classList.contains('is-staggered'));
    assert.ok(annotationElements[1].classList.contains('is-staggered'));
    assert.notOk(annotationElements[2].classList.contains('is-staggered'));

    await componentA11yAudit(this.element, assert);
  });

  test('horizontal annotations render in order', async function (assert) {
    const annotations = [
      { y: 2, label: 'label one' },
      { y: 9, label: 'label three' },
      { y: 2.4, label: 'label two' },
    ];
    this.setProperties({
      annotations,
      data: [
        { x: 1, y: 1 },
        { x: 10, y: 10 },
      ],
    });

    await render(
      <template>
        <LineChart @xProp="x" @yProp="y" @data={{this.data}}>
          <:after as |c|>
            <c.HAnnotations
              @annotations={{this.annotations}}
              @labelProp="label"
            />
          </:after>
        </LineChart>
      </template>,
    );

    const annotationElements = findAll('[data-test-annotation]');
    [...annotations]
      .sort((a, b) => (b.y || 0) - (a.y || 0))
      .forEach((annotation, index) => {
        assert.deepEqual(
          annotationElements[index].textContent.trim(),
          annotation.label,
        );
      });
  });

  test('the tooltip includes information on the data closest to the mouse', async function (assert) {
    const series1 = [
      { x: 1, y: 2 },
      { x: 3, y: 3 },
      { x: 5, y: 4 },
    ];
    const series2 = [
      { x: 2, y: 10 },
      { x: 4, y: 9 },
      { x: 6, y: 8 },
    ];
    this.setProperties({
      data: [
        { series: 'One', data: series1 },
        { series: 'Two', data: series2 },
      ],
    });

    await render(
      <template>
        <div style="width:500px;margin-top:100px">
          <LineChart @xProp="x" @yProp="y" @dataProp="data" @data={{this.data}}>
            <:svg as |c|>
              {{#each this.data as |series index|}}
                <c.Area
                  @data={{series.data}}
                  @colorScale="blues"
                  @index={{index}}
                />
              {{/each}}
            </:svg>
            <:after as |c|>
              <c.Tooltip as |series datum index|>
                <li>
                  <span class="label"><span
                      class="color-swatch swatch-blues swatch-blues-{{index}}"
                    />{{series.series}}</span>
                  <span class="value">{{datum.formattedY}}</span>
                </li>
              </c.Tooltip>
            </:after>
          </LineChart>
        </div>
      </template>,
    );

    const hoverTarget = find('[data-test-hover-target]');

    const bbox = hoverTarget.getBoundingClientRect();
    const xOffset = bbox.x;
    const interval = bbox.width / 5;

    await triggerEvent(hoverTarget, 'mouseenter');
    await triggerEvent(hoverTarget, 'mousemove', {
      clientX: xOffset + interval * 1 + 5,
    });
    assert.deepEqual(findAll('[data-test-chart-tooltip] li').length, 1);
    assert.deepEqual(
      find('[data-test-chart-tooltip] .label').textContent.trim(),
      this.data[1].series,
    );
    assert.deepEqual(
      find('[data-test-chart-tooltip] .value').textContent.trim(),
      String(series2.find((datum) => datum.x === 2).y),
    );

    const expected = [
      {
        label: this.data[0].series,
        value: series1.find((datum) => datum.x === 3).y,
      },
      {
        label: this.data[1].series,
        value: series2.find((datum) => datum.x === 2).y,
      },
    ];
    await triggerEvent(hoverTarget, 'mousemove', {
      clientX: xOffset + interval * 1.5 + 5,
    });
    assert.deepEqual(findAll('[data-test-chart-tooltip] li').length, 2);
    findAll('[data-test-chart-tooltip] li').forEach((tooltipEntry, index) => {
      assert.deepEqual(
        tooltipEntry.querySelector('.label').textContent.trim(),
        expected[index].label,
      );
      assert.deepEqual(
        tooltipEntry.querySelector('.value').textContent.trim(),
        String(expected[index].value),
      );
    });
  });
});
