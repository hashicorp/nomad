/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { findAll, render } from '@ember/test-helpers';
import { setupRenderingTest } from 'ember-qunit';
import { module, test } from 'qunit';
import ServiceStatusBar from 'nomad-ui/components/service-status-bar';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | Service Status Bar', function (hooks) {
  setupRenderingTest(hooks);

  test('Visualizes aggregate status of a service', async function (assert) {
    this.serviceStatus = {
      success: 1,
      pending: 1,
      failure: 1,
    };

    await render(
      <template>
        <div class="inline-chart">
          <ServiceStatusBar @status={{this.serviceStatus}} @name="peter" />
        </div>
      </template>,
    );

    await componentA11yAudit(this.element, assert);
    const bars = findAll('g > g').length;

    assert.deepEqual(bars, 3, 'It visualizes services by status');
  });
});
