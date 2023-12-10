/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import generateResources from '../../../mirage/data/generate-resources';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { find, render } from '@ember/test-helpers';
import Response from 'ember-cli-mirage/response';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';

module('Integration | Component | allocation row', function (hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    fragmentSerializerInitializer(this.owner);
    this.store = this.owner.lookup('service:store');
    this.server = startMirage();
    this.server.create('namespace');
    this.server.create('node-pool');
    this.server.create('node');
    this.server.create('job', { createAllocations: false });
  });

  hooks.afterEach(function () {
    this.server.shutdown();
  });

  test('Allocation row polls for stats, even when it errors or has an invalid response', async function (assert) {
    const component = this;

    let currentFrame = 0;
    let frames = [
      JSON.stringify({ ResourceUsage: generateResources() }),
      JSON.stringify({ ResourceUsage: generateResources() }),
      null,
      '<Not>Valid JSON</Not>',
      JSON.stringify({ ResourceUsage: generateResources() }),
    ];

    this.server.get('/client/allocation/:id/stats', function () {
      const response = frames[++currentFrame];

      // Disable polling to stop the EC task in the component
      if (currentFrame >= frames.length) {
        component.set('enablePolling', false);
      }

      if (response) {
        return response;
      }
      return new Response(500, {}, '');
    });

    this.server.create('allocation', { clientStatus: 'running' });
    await this.store.findAll('allocation');

    const allocation = this.store.peekAll('allocation').get('firstObject');

    this.setProperties({
      allocation,
      context: 'job',
      enablePolling: true,
    });

    await render(hbs`
      <AllocationRow
        @allocation={{allocation}}
        @context={{context}}
        @enablePolling={{enablePolling}} />
    `);

    assert.equal(
      this.server.pretender.handledRequests.filterBy(
        'url',
        `/v1/client/allocation/${allocation.get('id')}/stats`
      ).length,
      frames.length,
      'Requests continue to be made after malformed responses and server errors'
    );
  });

  test('Allocation row shows warning when it requires drivers that are unhealthy on the node it is running on', async function (assert) {
    assert.expect(2);

    const node = this.server.schema.nodes.first();
    const drivers = node.drivers;
    Object.values(drivers).forEach((driver) => {
      driver.Healthy = false;
      driver.Detected = true;
    });
    node.update({ drivers });

    this.server.create('allocation', { clientStatus: 'running' });
    await this.store.findAll('job');
    await this.store.findAll('node');
    await this.store.findAll('allocation');

    const allocation = this.store.peekAll('allocation').get('firstObject');

    this.setProperties({
      allocation,
      context: 'job',
    });

    await render(hbs`
      <AllocationRow
        @allocation={{allocation}}
        @context={{context}} />
    `);

    assert.ok(
      find('[data-test-icon="unhealthy-driver"]'),
      'Unhealthy driver icon is shown'
    );
    await componentA11yAudit(this.element, assert);
  });

  test('Allocation row shows an icon indicator when it was preempted', async function (assert) {
    assert.expect(2);

    const allocId = this.server.create('allocation', 'preempted').id;
    const allocation = await this.store.findRecord('allocation', allocId);

    this.setProperties({ allocation, context: 'job' });
    await render(hbs`
      <AllocationRow
        @allocation={{allocation}}
        @context={{context}} />
    `);

    assert.ok(find('[data-test-icon="preemption"]'), 'Preempted icon is shown');
    await componentA11yAudit(this.element, assert);
  });

  test('when an allocation is not running, the utilization graphs are omitted', async function (assert) {
    assert.expect(8);

    this.setProperties({
      context: 'job',
      enablePolling: false,
    });

    // All non-running statuses need to be tested
    ['pending', 'complete', 'failed', 'lost'].forEach((clientStatus) =>
      this.server.create('allocation', { clientStatus })
    );

    await this.store.findAll('allocation');

    const allocations = this.store.peekAll('allocation');

    for (const allocation of allocations.toArray()) {
      this.set('allocation', allocation);
      await render(hbs`
          <AllocationRow
            @allocation={{allocation}}
            @context={{context}}
            @enablePolling={{enablePolling}} />
        `);

      const status = allocation.get('clientStatus');
      assert.notOk(
        find('[data-test-cpu] .inline-chart'),
        `No CPU chart for ${status}`
      );
      assert.notOk(
        find('[data-test-mem] .inline-chart'),
        `No Mem chart for ${status}`
      );
    }
  });
});
