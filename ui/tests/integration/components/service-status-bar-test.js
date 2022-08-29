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

    const healthyService = {
      success: 1,
    };

    const failingService = {
      failure: 1,
    };

    const pendingService = {
      pending: 1,
    };

    const services = new Map();
    services.set('peter', healthyService);
    services.set('peter', { ...services.get('peter'), ...failingService });
    services.set('peter', { ...services.get('peter'), ...pendingService });

    this.set('services', services);

    await render(hbs`
      <div class="inline-chart">
        <ServiceStatusBar
          @services={{this.services}}
          @name="peter"
        />
      </div>
    `);

    const bars = findAll('g > g').length;

    assert.equal(bars, 3, 'It visualizes services by status');
  });
});
