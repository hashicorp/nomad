/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { find, render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import { create } from 'ember-cli-page-object';
import gaugeChart from 'nomad-ui/tests/pages/components/gauge-chart';

const GaugeChart = create(gaugeChart());

module('Integration | Component | gauge chart', function (hooks) {
  setupRenderingTest(hooks);

  const commonProperties = () => ({
    value: 5,
    total: 10,
    label: 'Gauge',
  });

  test('presents as an svg, a formatted percentage, and a label', async function (assert) {
    assert.expect(4);

    const props = commonProperties();
    this.setProperties(props);

    await render(hbs`
      <GaugeChart
        @value={{value}}
        @total={{total}}
        @label={{label}} />
    `);

    assert.equal(GaugeChart.label, props.label);
    assert.equal(GaugeChart.percentage, '50%');
    assert.ok(GaugeChart.svgIsPresent);

    await componentA11yAudit(this.element, assert);
  });

  test('the width of the chart is determined based on the container and the height is a function of the width', async function (assert) {
    const props = commonProperties();
    this.setProperties(props);

    await render(hbs`
      <div style="width:100px">
        <GaugeChart
          @value={{value}}
          @total={{total}}
          @label={{label}} />
      </div>
    `);

    const svg = find('[data-test-gauge-svg]');

    assert.equal(window.getComputedStyle(svg).width, '100px');
    assert.equal(svg.getAttribute('height'), 50);
  });
});
