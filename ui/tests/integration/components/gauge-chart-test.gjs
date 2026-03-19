/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { find, render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import { create } from 'ember-cli-page-object';
import GaugeChartComponent from 'nomad-ui/components/gauge-chart';
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
    const props = commonProperties();

    await render(
      <template>
        <GaugeChartComponent
          @value={{props.value}}
          @total={{props.total}}
          @label={{props.label}}
        />
      </template>,
    );

    assert.deepEqual(GaugeChart.label, props.label);
    assert.deepEqual(GaugeChart.percentage, '50%');
    assert.ok(GaugeChart.svgIsPresent);

    await componentA11yAudit(find('.chart.gauge-chart'), assert);
  });

  test('the width of the chart is determined based on the container and the height is a function of the width', async function (assert) {
    const props = commonProperties();

    await render(
      <template>
        <div style="width:100px">
          <GaugeChartComponent
            @value={{props.value}}
            @total={{props.total}}
            @label={{props.label}}
          />
        </div>
      </template>,
    );

    const svg = find('[data-test-gauge-svg]');

    assert.deepEqual(window.getComputedStyle(svg).width, '100px');
    assert.strictEqual(svg.getAttribute('height'), '50');
  });
});
