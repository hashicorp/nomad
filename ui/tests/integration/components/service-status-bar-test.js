/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { findAll, render } from '@ember/test-helpers';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { module, test } from 'qunit';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | Service Status Bar', function (hooks) {
  setupRenderingTest(hooks);

  test('Visualizes aggregate status of a service', async function (assert) {
    assert.expect(2);
    const component = this;
    await componentA11yAudit(component, assert);

    const serviceStatus = {
      success: 1,
      pending: 1,
      failure: 1,
    };

    this.set('serviceStatus', serviceStatus);

    await render(hbs`
      <div class="inline-chart">
        <ServiceStatusBar
          @status={{this.serviceStatus}}
          @name="peter"
        />
      </div>
    `);

    const bars = findAll('g > g').length;

    assert.equal(bars, 3, 'It visualizes services by status');
  });
});
