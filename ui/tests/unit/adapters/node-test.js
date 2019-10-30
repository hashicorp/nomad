import { run } from '@ember/runloop';
import { module, test } from 'qunit';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import { setupTest } from 'ember-qunit';
import { settled } from '@ember/test-helpers';

module('Unit | Adapter | Node', function(hooks) {
  setupTest(hooks);

  hooks.beforeEach(function() {
    this.store = this.owner.lookup('service:store');
    this.subject = () => this.store.adapterFor('node');

    this.server = startMirage();
    this.server.create('node', { id: 'node-1' });
    this.server.create('node', { id: 'node-2' });
    this.server.create('job', { id: 'job-1', createAllocations: false });

    this.server.create('allocation', { id: 'node-1-1', nodeId: 'node-1' });
    this.server.create('allocation', { id: 'node-1-2', nodeId: 'node-1' });
    this.server.create('allocation', { id: 'node-2-1', nodeId: 'node-2' });
    this.server.create('allocation', { id: 'node-2-2', nodeId: 'node-2' });
  });

  hooks.afterEach(function() {
    this.server.shutdown();
  });

  test('findHasMany removes old related models from the store', async function(assert) {
    // Fetch the model and related allocations
    let node = await run(() => this.store.findRecord('node', 'node-1'));
    let allocations = await run(() => findHasMany(node, 'allocations'));
    assert.equal(
      allocations.get('length'),
      this.server.db.allocations.where({ nodeId: node.get('id') }).length,
      'Allocations returned from the findHasMany matches the db state'
    );

    await settled();
    server.db.allocations.remove('node-1-1');

    allocations = await run(() => findHasMany(node, 'allocations'));
    const dbAllocations = this.server.db.allocations.where({ nodeId: node.get('id') });
    assert.equal(
      allocations.get('length'),
      dbAllocations.length,
      'Allocations returned from the findHasMany matches the db state'
    );
    assert.equal(
      this.store.peekAll('allocation').get('length'),
      dbAllocations.length,
      'Server-side deleted allocation was removed from the store'
    );
  });

  test('findHasMany does not remove old unrelated models from the store', async function(assert) {
    // Fetch the first node and related allocations
    const node = await run(() => this.store.findRecord('node', 'node-1'));
    await run(() => findHasMany(node, 'allocations'));

    // Also fetch the second node and related allocations;
    const node2 = await run(() => this.store.findRecord('node', 'node-2'));
    await run(() => findHasMany(node2, 'allocations'));

    await settled();
    assert.deepEqual(
      this.store
        .peekAll('allocation')
        .mapBy('id')
        .sort(),
      ['node-1-1', 'node-1-2', 'node-2-1', 'node-2-2'],
      'All allocations for the first and second node are in the store'
    );

    server.db.allocations.remove('node-1-1');

    // Reload the related allocations now that one was removed server-side
    await run(() => findHasMany(node, 'allocations'));
    assert.deepEqual(
      this.store
        .peekAll('allocation')
        .mapBy('id')
        .sort(),
      ['node-1-2', 'node-2-1', 'node-2-2'],
      'The deleted allocation is removed from the store and the allocations associated with the other node are untouched'
    );
  });

  test('setEligible makes the correct POST request to /:node_id/eligibility', async function(assert) {
    const { pretender } = this.server;
    const node = await run(() => this.store.findRecord('node', 'node-1'));

    await this.subject().setEligible(node);

    const request = pretender.handledRequests.lastObject;
    assert.equal(
      request.url,
      `/v1/node/${node.id}/eligibility`,
      'Request was made to /:node_id/eligibility'
    );
    assert.equal(request.method, 'POST', 'Request was made with the POST method');
    assert.deepEqual(
      JSON.parse(request.requestBody),
      {
        NodeID: node.id,
        Eligibility: 'eligible',
      },
      'POST request is made with the correct body arguments'
    );
  });

  test('setIneligible makes the correct POST request to /:node_id/eligibility', async function(assert) {
    const { pretender } = this.server;
    const node = await run(() => this.store.findRecord('node', 'node-1'));

    await this.subject().setIneligible(node);

    const request = pretender.handledRequests.lastObject;
    assert.equal(
      request.url,
      `/v1/node/${node.id}/eligibility`,
      'Request was made to /:node_id/eligibility'
    );
    assert.equal(request.method, 'POST', 'Request was made with the POST method');
    assert.deepEqual(
      JSON.parse(request.requestBody),
      {
        NodeID: node.id,
        Eligibility: 'ineligible',
      },
      'POST request is made with the correct body arguments'
    );
  });

  test('drain makes the correct POST request to /:node_id/drain with appropriate defaults', async function(assert) {
    const { pretender } = this.server;
    const node = await run(() => this.store.findRecord('node', 'node-1'));

    await this.subject().drain(node);

    const request = pretender.handledRequests.lastObject;
    assert.equal(request.url, `/v1/node/${node.id}/drain`, 'Request was made to /:node_id/drain');
    assert.equal(request.method, 'POST', 'Request was made with the POST method');
    assert.deepEqual(
      JSON.parse(request.requestBody),
      {
        NodeID: node.id,
        DrainSpec: {
          Deadline: 0,
          IgnoreSystemJobs: true,
        },
      },
      'POST request is made with the default body arguments'
    );
  });

  test('drain makes the correct POST request to /:node_id/drain with the provided drain spec', async function(assert) {
    const { pretender } = this.server;
    const node = await run(() => this.store.findRecord('node', 'node-1'));

    const spec = { Deadline: 123456789, IgnoreSystemJobs: false };
    await this.subject().drain(node, spec);

    const request = pretender.handledRequests.lastObject;
    assert.deepEqual(
      JSON.parse(request.requestBody),
      {
        NodeID: node.id,
        DrainSpec: {
          Deadline: spec.Deadline,
          IgnoreSystemJobs: spec.IgnoreSystemJobs,
        },
      },
      'POST request is made with the drain spec as body arguments'
    );
  });

  test('forceDrain makes the correct POST request to /:node_id/drain with appropriate defaults', async function(assert) {
    const { pretender } = this.server;
    const node = await run(() => this.store.findRecord('node', 'node-1'));

    await this.subject().forceDrain(node);

    const request = pretender.handledRequests.lastObject;
    assert.equal(request.url, `/v1/node/${node.id}/drain`, 'Request was made to /:node_id/drain');
    assert.equal(request.method, 'POST', 'Request was made with the POST method');
    assert.deepEqual(
      JSON.parse(request.requestBody),
      {
        NodeID: node.id,
        DrainSpec: {
          Deadline: -1000000000,
          IgnoreSystemJobs: true,
        },
      },
      'POST request is made with the default body arguments'
    );
  });

  test('forceDrain makes the correct POST request to /:node_id/drain with the provided drain spec', async function(assert) {
    const { pretender } = this.server;
    const node = await run(() => this.store.findRecord('node', 'node-1'));

    const spec = { Deadline: 123456789, IgnoreSystemJobs: false };
    await this.subject().forceDrain(node, spec);

    const request = pretender.handledRequests.lastObject;
    assert.equal(request.url, `/v1/node/${node.id}/drain`, 'Request was made to /:node_id/drain');
    assert.equal(request.method, 'POST', 'Request was made with the POST method');
    assert.deepEqual(
      JSON.parse(request.requestBody),
      {
        NodeID: node.id,
        DrainSpec: {
          Deadline: -1000000000,
          IgnoreSystemJobs: spec.IgnoreSystemJobs,
        },
      },
      'POST request is made with the drain spec, except deadline is not overridden'
    );
  });
});

// Using fetchLink on a model's hasMany relationship exercises the adapter's
// findHasMany method as well normalizing the response and pushing it to the store
function findHasMany(model, relationshipName) {
  const relationship = model.relationshipFor(relationshipName);
  return model.hasMany(relationship.key).reload();
}
