import { A } from '@ember/array';
import EmberObject from '@ember/object';
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

    const healthyService = EmberObject.create({
      id: '1',
      status: 'success',
    });

    const failingService = EmberObject.create({
      id: '2',
      status: 'failing',
    });

    const pendingService = EmberObject.create({
      id: '3',
      status: 'pending',
    });

    const services = A([healthyService, failingService, pendingService]);
    this.set('services', services);

    await render(hbs`
      <div class="inline-chart">
        <ServiceStatusBar
          @services={{this.services}}  
        />
      </div>
    `);

    const bars = findAll('g > g').length;

    assert.equal(bars, 3, 'It visualizes services by status');
  });
});
