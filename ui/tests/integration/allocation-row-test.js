import { getOwner } from '@ember/application';
import { test, moduleForComponent } from 'ember-qunit';
import wait from 'ember-test-helpers/wait';
import hbs from 'htmlbars-inline-precompile';
import generateResources from '../../mirage/data/generate-resources';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { find } from 'ember-native-dom-helpers';
import Response from 'ember-cli-mirage/response';
import { initialize as fragmentSerializerInitializer } from 'nomad-ui/initializers/fragment-serializer';
import { Promise, resolve } from 'rsvp';

moduleForComponent('allocation-row', 'Integration | Component | allocation row', {
  integration: true,
  beforeEach() {
    fragmentSerializerInitializer(getOwner(this));
    this.store = getOwner(this).lookup('service:store');
    this.server = startMirage();
    this.server.create('namespace');
    this.server.create('node');
    this.server.create('job', { createAllocations: false });
  },
  afterEach() {
    this.server.shutdown();
  },
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

  return wait()
    .then(() => {
      allocation = this.store.peekAll('allocation').get('firstObject');

      this.setProperties({
        allocation,
        context: 'job',
        enablePolling: true,
      });

      this.render(hbs`
        {{allocation-row
          allocation=allocation
          context=context
          enablePolling=enablePolling}}
      `);
      return wait();
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

  return wait()
    .then(() => {
      allocation = this.store.peekAll('allocation').get('firstObject');

      this.setProperties({
        allocation,
        context: 'job',
      });

      this.render(hbs`
        {{allocation-row
          allocation=allocation
          context=context}}
      `);
      return wait();
    })
    .then(() => {
      assert.ok(find('[data-test-icon="unhealthy-driver"]'), 'Unhealthy driver icon is shown');
    });
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

  return wait().then(() => {
    const allocations = this.store.peekAll('allocation');
    return waitForEach(
      allocations.map(allocation => () => {
        this.set('allocation', allocation);
        this.render(hbs`
            {{allocation-row
              allocation=allocation
              context=context
              enablePolling=enablePolling}}
          `);
        return wait().then(() => {
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
    promise.then(() => wait()).then(step);
  };

  step();

  return pending;
};
