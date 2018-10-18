import { run } from '@ember/runloop';
import { test } from 'ember-qunit';
import wait from 'ember-test-helpers/wait';
import { startMirage } from 'nomad-ui/initializers/ember-cli-mirage';
import moduleForAdapter from '../../helpers/module-for-adapter';

moduleForAdapter('node', 'Unit | Adapter | Node', {
  needs: [
    'adapter:node',
    'model:node-attributes',
    'model:allocation',
    'model:node-driver',
    'model:node-event',
    'model:evaluation',
    'model:job',
    'serializer:application',
    'serializer:node',
    'service:system',
    'service:token',
    'service:config',
    'service:watchList',
    'transform:fragment',
    'transform:fragment-array',
  ],
  beforeEach() {
    this.server = startMirage();
    this.server.create('node', { id: 'node-1' });
    this.server.create('node', { id: 'node-2' });
    this.server.create('job', { id: 'job-1', createAllocations: false });

    this.server.create('allocation', { id: 'node-1-1', nodeId: 'node-1' });
    this.server.create('allocation', { id: 'node-1-2', nodeId: 'node-1' });
    this.server.create('allocation', { id: 'node-2-1', nodeId: 'node-2' });
    this.server.create('allocation', { id: 'node-2-2', nodeId: 'node-2' });
  },
  afterEach() {
    this.server.shutdown();
  },
});

test('findHasMany removes old related models from the store', function(assert) {
  let node;
  run(() => {
    // Fetch the model
    this.store.findRecord('node', 'node-1').then(model => {
      node = model;

      // Fetch the related allocations
      return findHasMany(model, 'allocations').then(allocations => {
        assert.equal(
          allocations.get('length'),
          this.server.db.allocations.where({ nodeId: node.get('id') }).length,
          'Allocations returned from the findHasMany matches the db state'
        );
      });
    });
  });

  return wait().then(() => {
    server.db.allocations.remove('node-1-1');

    run(() => {
      // Reload the related allocations now that one was removed server-side
      return findHasMany(node, 'allocations').then(allocations => {
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
    });
  });
});

test('findHasMany does not remove old unrelated models from the store', function(assert) {
  let node;

  run(() => {
    // Fetch the first node and related allocations
    this.store.findRecord('node', 'node-1').then(model => {
      node = model;
      return findHasMany(model, 'allocations');
    });

    // Also fetch the second node and related allocations;
    this.store.findRecord('node', 'node-2').then(model => findHasMany(model, 'allocations'));
  });

  return wait().then(() => {
    assert.deepEqual(
      this.store
        .peekAll('allocation')
        .mapBy('id')
        .sort(),
      ['node-1-1', 'node-1-2', 'node-2-1', 'node-2-2'],
      'All allocations for the first and second node are in the store'
    );

    server.db.allocations.remove('node-1-1');

    run(() => {
      // Reload the related allocations now that one was removed server-side
      return findHasMany(node, 'allocations').then(() => {
        assert.deepEqual(
          this.store
            .peekAll('allocation')
            .mapBy('id')
            .sort(),
          ['node-1-2', 'node-2-1', 'node-2-2'],
          'The deleted allocation is removed from the store and the allocations associated with the other node are untouched'
        );
      });
    });
  });
});

// Using fetchLink on a model's hasMany relationship exercises the adapter's
// findHasMany method as well normalizing the response and pushing it to the store
function findHasMany(model, relationshipName) {
  const relationship = model.relationshipFor(relationshipName);
  return model.hasMany(relationship.key).hasManyRelationship.fetchLink();
}
