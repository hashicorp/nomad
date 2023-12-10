/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, triggerEvent } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | das/recommendation-chart', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders a chart for a recommended CPU increase', async function (assert) {
    assert.expect(5);

    this.set('resource', 'CPU');
    this.set('current', 1312);
    this.set('recommended', 1919);
    this.set('stats', {});

    await render(
      hbs`<Das::RecommendationChart
            @resource={{resource}}
            @currentValue={{current}}
            @recommendedValue={{recommended}}
            @stats={{stats}}
          />`
    );

    assert.dom('.recommendation-chart.increase').exists();
    assert.dom('.recommendation-chart .resource').hasText('CPU');
    assert.dom('.recommendation-chart .icon-is-arrow-up').exists();
    assert.dom('text.percent').hasText('+46%');
    await componentA11yAudit(this.element, assert);
  });

  test('it renders a chart for a recommended memory decrease', async function (assert) {
    assert.expect(5);

    this.set('resource', 'MemoryMB');
    this.set('current', 1919);
    this.set('recommended', 1312);
    this.set('stats', {});

    await render(
      hbs`<Das::RecommendationChart
            @resource={{resource}}
            @currentValue={{current}}
            @recommendedValue={{recommended}}
            @stats={{stats}}
          />`
    );

    assert.dom('.recommendation-chart.decrease').exists();
    assert.dom('.recommendation-chart .resource').hasText('Mem');
    assert.dom('.recommendation-chart .icon-is-arrow-down').exists();
    assert.dom('text.percent').hasText('−32%');
    await componentA11yAudit(this.element, assert);
  });

  test('it handles the maximum being far beyond the recommended', async function (assert) {
    this.set('resource', 'CPU');
    this.set('current', 1312);
    this.set('recommended', 1919);
    this.set('stats', {
      max: 3000,
    });

    await render(
      hbs`<Das::RecommendationChart
            @resource={{resource}}
            @currentValue={{current}}
            @recommendedValue={{recommended}}
            @stats={{stats}}
          />`
    );

    const chartSvg = this.element.querySelector('.recommendation-chart svg');
    const maxLine = chartSvg.querySelector('line.stat.max');

    assert.ok(maxLine.getAttribute('x1') < chartSvg.clientWidth);
  });

  test('it can be disabled and will show no delta', async function (assert) {
    assert.expect(6);

    this.set('resource', 'CPU');
    this.set('current', 1312);
    this.set('recommended', 1919);
    this.set('stats', {});

    await render(
      hbs`<Das::RecommendationChart
            @resource={{resource}}
            @currentValue={{current}}
            @recommendedValue={{recommended}}
            @stats={{stats}}
            @disabled={{true}}
          />`
    );

    assert.dom('.recommendation-chart.disabled');
    assert.dom('.recommendation-chart.increase').doesNotExist();
    assert.dom('.recommendation-chart rect.delta').doesNotExist();
    assert.dom('.recommendation-chart .changes').doesNotExist();
    assert.dom('.recommendation-chart .resource').hasText('CPU');
    assert.dom('.recommendation-chart .icon-is-arrow-up').exists();
    await componentA11yAudit(this.element, assert);
  });

  test('the stats labels shift aligment and disappear to account for space', async function (assert) {
    this.set('resource', 'CPU');
    this.set('current', 50);
    this.set('recommended', 100);

    this.set('stats', {
      mean: 5,
      p99: 99,
      max: 100,
    });

    await render(
      hbs`<Das::RecommendationChart
            @resource={{resource}}
            @currentValue={{current}}
            @recommendedValue={{recommended}}
            @stats={{stats}}
          />`
    );

    assert.dom('[data-test-label=max]').hasClass('right');

    this.set('stats', {
      mean: 5,
      p99: 6,
      max: 100,
    });

    assert.dom('[data-test-label=max]').hasNoClass('right');
    assert.dom('[data-test-label=p99]').hasClass('right');

    this.set('stats', {
      mean: 5,
      p99: 6,
      max: 7,
    });

    assert.dom('[data-test-label=max]').hasClass('right');
    assert.dom('[data-test-label=p99]').hasClass('hidden');
  });

  test('a legend tooltip shows the sorted stats values on hover', async function (assert) {
    this.set('resource', 'CPU');
    this.set('current', 50);
    this.set('recommended', 101);

    this.set('stats', {
      mean: 5,
      p99: 99,
      max: 100,
      min: 1,
      median: 55,
    });

    await render(
      hbs`<Das::RecommendationChart
            @resource={{resource}}
            @currentValue={{current}}
            @recommendedValue={{recommended}}
            @stats={{stats}}
          />`
    );

    assert.dom('.chart-tooltip').isNotVisible();

    await triggerEvent('.recommendation-chart', 'mousemove');

    assert.dom('.chart-tooltip').isVisible();

    assert.dom('.chart-tooltip li:nth-child(1)').hasText('Min 1');
    assert.dom('.chart-tooltip li:nth-child(2)').hasText('Mean 5');
    assert.dom('.chart-tooltip li:nth-child(3)').hasText('Current 50');
    assert.dom('.chart-tooltip li:nth-child(4)').hasText('Median 55');
    assert.dom('.chart-tooltip li:nth-child(5)').hasText('99th 99');
    assert.dom('.chart-tooltip li:nth-child(6)').hasText('Max 100');
    assert.dom('.chart-tooltip li:nth-child(7)').hasText('New 101');

    assert.dom('.chart-tooltip li.active').doesNotExist();

    await triggerEvent('.recommendation-chart text.changes.new', 'mouseenter');
    assert.dom('.chart-tooltip li:nth-child(7).active').exists();

    await triggerEvent('.recommendation-chart line.stat.max', 'mouseenter');
    assert.dom('.chart-tooltip li:nth-child(6).active').exists();

    await triggerEvent('.recommendation-chart rect.stat.p99', 'mouseenter');
    assert.dom('.chart-tooltip li:nth-child(5).active').exists();

    await triggerEvent('.recommendation-chart', 'mouseleave');

    assert.dom('.chart-tooltip').isNotVisible();
  });
});
