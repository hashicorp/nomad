import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, settled } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';
import generateResources from '../../mirage/data/generate-resources';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { find } from '@ember/test-helpers';
import Response from 'ember-cli-mirage/response';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';
import { Promise, resolve } from 'rsvp';

module('Integration | Component | allocation row', function(hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function() {
    fragmentSerializerInitializer(this.owner);
    this.store = this.owner.lookup('service:store');
    this.server = startMirage();
    this.server.create('namespace');
    this.server.create('node');
    this.server.create('job', { createAllocations: false });
  });

  hooks.afterEach(function() {
    this.server.shutdown();
  });

  test('Allocation row polls for stats, even when it errors or has an invalid response', function(assert) {
    const component = this;

    let currentFrame = 0;
    let frames = [
      JSON.stringify({ ResourceUsage: generateResources() }),
      JSON.stringify({ ResourceUsage: generateResources() }),
      null,
      '<Not>Valid JSON</Not>',
      JSON.stringify({ ResourceUsage: generateResources() }),
    ];

    this.server.get('/client/allocation/:id/stats', function() {
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
    this.store.findAll('allocation');

    let allocation;

    return settled()
      .then(async () => {
        allocation = this.store.peekAll('allocation').get('firstObject');

        this.setProperties({
          allocation,
          context: 'job',
          enablePolling: true,
        });

        await render(hbs`
          {{allocation-row
            allocation=allocation
            context=context
            enablePolling=enablePolling}}
        `);
        return settled();
      })
      .then(() => {
        assert.equal(
          this.server.pretender.handledRequests.filterBy(
            'url',
            `/v1/client/allocation/${allocation.get('id')}/stats`
          ).length,
          frames.length,
          'Requests continue to be made after malformed responses and server errors'
        );
      });
  });

  test('Allocation row shows warning when it requires drivers that are unhealthy on the node it is running on', function(assert) {
    const node = this.server.schema.nodes.first();
    const drivers = node.drivers;
    Object.values(drivers).forEach(driver => {
      driver.Healthy = false;
      driver.Detected = true;
    });
    node.update({ drivers });

    this.server.create('allocation', { clientStatus: 'running' });
    this.store.findAll('job');
    this.store.findAll('node');
    this.store.findAll('allocation');

    let allocation;

    return settled()
      .then(async () => {
        allocation = this.store.peekAll('allocation').get('firstObject');

        this.setProperties({
          allocation,
          context: 'job',
        });

        await render(hbs`
          {{allocation-row
            allocation=allocation
            context=context}}
        `);
        return settled();
      })
      .then(() => {
        assert.ok(find('[data-test-icon="unhealthy-driver"]'), 'Unhealthy driver icon is shown');
      });
  });

  test('Allocation row shows an icon indicator when it was preempted', async function(assert) {
    const allocId = this.server.create('allocation', 'preempted').id;

    const allocation = await this.store.findRecord('allocation', allocId);
    await settled();

    this.setProperties({ allocation, context: 'job' });
    await render(hbs`
      {{allocation-row
        allocation=allocation
        context=context}}
    `);
    await settled();

    assert.ok(find('[data-test-icon="preemption"]'), 'Preempted icon is shown');
  });

  test('when an allocation is not running, the utilization graphs are omitted', function(assert) {
    this.setProperties({
      context: 'job',
      enablePolling: false,
    });

    // All non-running statuses need to be tested
    ['pending', 'complete', 'failed', 'lost'].forEach(clientStatus =>
      this.server.create('allocation', { clientStatus })
    );

    this.store.findAll('allocation');

    return settled().then(() => {
      const allocations = this.store.peekAll('allocation');
      return waitForEach(
        allocations.map(allocation => async () => {
          this.set('allocation', allocation);
          await render(hbs`
              {{allocation-row
                allocation=allocation
                context=context
                enablePolling=enablePolling}}
            `);
          return settled().then(() => {
            const status = allocation.get('clientStatus');
            assert.notOk(find('[data-test-cpu] .inline-chart'), `No CPU chart for ${status}`);
            assert.notOk(find('[data-test-mem] .inline-chart'), `No Mem chart for ${status}`);
          });
        })
      );
    });
  });

  // A way to loop over asynchronous code. Can be replaced by async/await in the future.
  const waitForEach = fns => {
    let i = 0;
    let done = () => {};

    // This function is asynchronous and needs to return a promise
    const pending = new Promise(resolve => {
      done = resolve;
    });

    const step = () => {
      // The waitForEach promise and this recursive loop are done once
      // all functions have been called.
      if (i >= fns.length) {
        done();
        return;
      }
      // Call the current function
      const promise = fns[i]() || resolve(true);
      // Increment the function position
      i++;
      // Wait for async behaviors to settle and repeat
      promise.then(() => settled()).then(step);
    };

    step();

    return pending;
  };
});
