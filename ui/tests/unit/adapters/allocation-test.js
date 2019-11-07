import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';

module('Unit | Adapter | Allocation', function(hooks) {
  setupTest(hooks);

  hooks.beforeEach(async function() {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.adapterFor('allocation');

    this.server = startMirage();

    this.server.create('namespace');
    this.server.create('node');
    this.server.create('job', { createAllocations: false });
    this.server.create('allocation', { id: 'alloc-1' });
  });

  hooks.afterEach(function() {
    this.server.shutdown();
  });

  test('`stop` makes the correct API call', async function(assert) {
    const { pretender } = this.server;
    const allocationId = 'alloc-1';

    const allocation = await this.store.findRecord('allocation', allocationId);
    pretender.handledRequests.length = 0;

    await this.subject().stop(allocation);
    const req = pretender.handledRequests[0];
    assert.equal(
      `${req.method} ${req.url}`,
      `POST /v1/allocation/${allocationId}/stop`,
      `POST /v1/allocation/${allocationId}/stop`
    );
  });

  test('`restart` makes the correct API call', async function(assert) {
    const { pretender } = this.server;
    const allocationId = 'alloc-1';

    const allocation = await this.store.findRecord('allocation', allocationId);
    pretender.handledRequests.length = 0;

    await this.subject().restart(allocation);
    const req = pretender.handledRequests[0];
    assert.equal(
      `${req.method} ${req.url}`,
      `PUT /v1/client/allocation/${allocationId}/restart`,
      `PUT /v1/client/allocation/${allocationId}/restart`
    );
  });

  test('`restart` takes an optional task name and makes the correct API call', async function(assert) {
    const { pretender } = this.server;
    const allocationId = 'alloc-1';
    const taskName = 'task-name';

    const allocation = await this.store.findRecord('allocation', allocationId);
    pretender.handledRequests.length = 0;

    await this.subject().restart(allocation, taskName);
    const req = pretender.handledRequests[0];
    assert.equal(
      `${req.method} ${req.url}`,
      `PUT /v1/client/allocation/${allocationId}/restart`,
      `PUT /v1/client/allocation/${allocationId}/restart`
    );
    assert.deepEqual(
      JSON.parse(req.requestBody),
      { TaskName: taskName },
      'Request body is correct'
    );
  });
});
