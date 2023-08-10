/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  find,
  findAll,
  click,
  render,
  triggerEvent,
} from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import sinon from 'sinon';
import moment from 'moment';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

const REF_DATE = new Date();

module('Integration | Component | line-chart', function (hooks) {
  setupRenderingTest(hooks);

  test('when a chart has annotations, they are rendered in order', async function (assert) {
    assert.expect(4);

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

    await render(hbs`
      <LineChart
        @xProp="x"
        @yProp="y"
        @data={{this.data}}>
        <:after as |c|>
          <c.VAnnotations @annotations={{this.annotations}} />
        </:after>
      </LineChart>
    `);

    const sortedAnnotations = annotations.sortBy('x');
    findAll('[data-test-annotation]').forEach((annotation, idx) => {
      const datum = sortedAnnotations[idx];
      assert.equal(
        annotation.querySelector('button').getAttribute('title'),
        `${datum.type} event at ${datum.x}`
      );
    });

    await componentA11yAudit(this.element, assert);
  });

  test('when a chart has annotations and is timeseries, annotations are sorted reverse-chronologically', async function (assert) {
    assert.expect(3);

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

    await render(hbs`
      <LineChart
        @xProp="x"
        @yProp="y"
        @timeseries={{true}}
        @data={{this.data}}>
        <:after as |c|>
          <c.VAnnotations @annotations={{this.annotations}} />
        </:after>
      </LineChart>
    `);

    const sortedAnnotations = annotations.sortBy('x').reverse();
    findAll('[data-test-annotation]').forEach((annotation, idx) => {
      const datum = sortedAnnotations[idx];
      assert.equal(
        annotation.querySelector('button').getAttribute('title'),
        `${datum.type} event at ${moment(datum.x).format('MMM DD, HH:mm')}`
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

    await render(hbs`
      <LineChart
        @xProp="x"
        @yProp="y"
        @data={{this.data}}>
        <:after as |c|>
          <c.VAnnotations @annotations={{this.annotations}} @annotationClick={{this.click}} />
        </:after>
      </LineChart>
    `);

    await click('[data-test-annotation] button');
    assert.ok(this.click.calledWith(annotations[0]));
  });

  test('annotations will have staggered heights when too close to be positioned side-by-side', async function (assert) {
    assert.expect(4);

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

    await render(hbs`
      <div style="width:200px;">
        <LineChart
          @xProp="x"
          @yProp="y"
          @data={{this.data}}>
          <:after as |c|>
            <c.VAnnotations @annotations={{this.annotations}} />
          </:after>
        </LineChart>
      </div>
    `);

    const annotationEls = findAll('[data-test-annotation]');
    assert.notOk(annotationEls[0].classList.contains('is-staggered'));
    assert.ok(annotationEls[1].classList.contains('is-staggered'));
    assert.notOk(annotationEls[2].classList.contains('is-staggered'));

    await componentA11yAudit(this.element, assert);
  });

  test('horizontal annotations render in order', async function (assert) {
    assert.expect(3);

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

    await render(hbs`
      <LineChart
        @xProp="x"
        @yProp="y"
        @data={{this.data}}>
        <:after as |c|>
          <c.HAnnotations @annotations={{this.annotations}} @labelProp="label" />
        </:after>
      </LineChart>
    `);

    const annotationEls = findAll('[data-test-annotation]');
    annotations
      .sortBy('y')
      .reverse()
      .forEach((annotation, index) => {
        assert.equal(annotationEls[index].textContent.trim(), annotation.label);
      });
  });

  test('the tooltip includes information on the data closest to the mouse', async function (assert) {
    assert.expect(8);

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

    await render(hbs`
      <div style="width:500px;margin-top:100px">
        <LineChart
          @xProp="x"
          @yProp="y"
          @dataProp="data"
          @data={{this.data}}>
          <:svg as |c|>
            {{#each this.data as |series idx|}}
              <c.Area @data={{series.data}} @colorScale="blues" @index={{idx}} />
            {{/each}}
          </:svg>
          <:after as |c|>
            <c.Tooltip as |series datum index|>
              <li>
                <span class="label"><span class="color-swatch swatch-blues swatch-blues-{{index}}" />{{series.series}}</span>
                <span class="value">{{datum.formattedY}}</span>
              </li>
            </c.Tooltip>
          </:after>
        </LineChart>
      </div>
    `);

    // All tooltip events are attached to the hover target
    const hoverTarget = find('[data-test-hover-target]');

    // Mouse to data mapping happens based on the clientX of the MouseEvent
    const bbox = hoverTarget.getBoundingClientRect();
    // The MouseEvent needs to be translated based on the location of the hover target
    const xOffset = bbox.x;
    // An interval here is the width between x values given the fixed dimensions of the line chart
    // and the domain of the data
    const interval = bbox.width / 5;

    // MouseEnter triggers the tooltip visibility
    await triggerEvent(hoverTarget, 'mouseenter');
    // MouseMove positions the tooltip and updates the active datum
    await triggerEvent(hoverTarget, 'mousemove', {
      clientX: xOffset + interval * 1 + 5,
    });
    assert.equal(findAll('[data-test-chart-tooltip] li').length, 1);
    assert.equal(
      find('[data-test-chart-tooltip] .label').textContent.trim(),
      this.data[1].series
    );
    assert.equal(
      find('[data-test-chart-tooltip] .value').textContent.trim(),
      series2.find((d) => d.x === 2).y
    );

    // When the mouse falls between points and each series has points with different x values,
    // points will only be shown in the tooltip if they are close enough to the closest point
    // to the cursor.
    // This event is intentionally between points such that both points are within proximity.
    const expected = [
      { label: this.data[0].series, value: series1.find((d) => d.x === 3).y },
      { label: this.data[1].series, value: series2.find((d) => d.x === 2).y },
    ];
    await triggerEvent(hoverTarget, 'mousemove', {
      clientX: xOffset + interval * 1.5 + 5,
    });
    assert.equal(findAll('[data-test-chart-tooltip] li').length, 2);
    findAll('[data-test-chart-tooltip] li').forEach((tooltipEntry, index) => {
      assert.equal(
        tooltipEntry.querySelector('.label').textContent.trim(),
        expected[index].label
      );
      assert.equal(
        tooltipEntry.querySelector('.value').textContent.trim(),
        expected[index].value
      );
    });
  });
});
