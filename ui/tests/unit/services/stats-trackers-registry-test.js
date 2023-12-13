/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import EmberObject from '@ember/object';
import Service from '@ember/service';
import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { settled } from '@ember/test-helpers';
import Pretender from 'pretender';
import sinon from 'sinon';
import fetch from 'nomad-ui/utils/fetch';
import NodeStatsTracker from 'nomad-ui/utils/classes/node-stats-tracker';

module('Unit | Service | Stats Trackers Registry', function (hooks) {
  setupTest(hooks);

  hooks.beforeEach(function () {
    this.subject = function () {
      return this.owner.factoryFor('service:stats-trackers-registry').create();
    };
  });

  hooks.beforeEach(function () {
    // Inject a mock token service
    const authorizedRequestSpy = (this.tokenAuthorizedRequestSpy = sinon.spy());
    const mockToken = Service.extend({
      authorizedRequest(url) {
        authorizedRequestSpy(url);
        return fetch(url);
      },
    });

    this.owner.register('service:token', mockToken);
    this.token = this.owner.lookup('service:token');
    this.server = new Pretender(function () {
      this.get('/v1/client/stats', () => [
        200,
        {},
        JSON.stringify({
          Timestamp: 1234567890,
          CPUTicksConsumed: 11,
          Memory: {
            Used: 12,
          },
        }),
      ]);
    });
  });

  hooks.afterEach(function () {
    this.server.shutdown();
  });

  const makeModelMock = (modelName, defaults) => {
    const Class = EmberObject.extend(defaults);
    Class.prototype.constructor.modelName = modelName;
    return Class;
  };

  const mockNode = makeModelMock('node', { id: 'test' });

  test('Creates a tracker when one isn’t found', function (assert) {
    const registry = this.subject();
    const id = 'id';

    assert.equal(
      registry.get('registryRef').size,
      0,
      'Nothing in the registry yet'
    );

    const tracker = registry.getTracker(mockNode.create({ id }));
    assert.ok(
      tracker instanceof NodeStatsTracker,
      'The correct type of tracker is made'
    );
    assert.equal(
      registry.get('registryRef').size,
      1,
      'The tracker was added to the registry'
    );
    assert.deepEqual(
      Array.from(registry.get('registryRef').keys()),
      [`node:${id}`],
      'The object in the registry has the correct key'
    );
  });

  test('Returns an existing tracker when one is found', function (assert) {
    const registry = this.subject();
    const node = mockNode.create();

    const tracker1 = registry.getTracker(node);
    const tracker2 = registry.getTracker(node);

    assert.equal(
      tracker1,
      tracker2,
      'Returns an existing tracker for the same resource'
    );
    assert.equal(
      registry.get('registryRef').size,
      1,
      'Only one tracker in the registry'
    );
  });

  test('Registry does not depend on persistent object references', function (assert) {
    const registry = this.subject();
    const id = 'some-id';

    const node1 = mockNode.create({ id });
    const node2 = mockNode.create({ id });

    assert.notEqual(node1, node2, 'Two different resources');
    assert.equal(node1.get('id'), node2.get('id'), 'With the same IDs');
    assert.equal(
      node1.constructor.modelName,
      node2.constructor.modelName,
      'And the same className'
    );

    assert.equal(
      registry.getTracker(node1),
      registry.getTracker(node2),
      'Return the same tracker'
    );
    assert.equal(
      registry.get('registryRef').size,
      1,
      'Only one tracker in the registry'
    );
  });

  test('Has a max size', function (assert) {
    const registry = this.subject();
    const ref = registry.get('registryRef');

    // Kind of a silly assertion, but the exact limit is arbitrary. Whether it's 10 or 1000
    // isn't important as long as there is one.
    assert.ok(ref.limit < Infinity, `A limit (${ref.limit}) is set`);
  });

  test('Registry re-attaches deleted resources to cached trackers', function (assert) {
    const registry = this.subject();
    const id = 'some-id';

    const node1 = mockNode.create({ id });
    let tracker = registry.getTracker(node1);

    assert.ok(tracker.get('node'), 'The tracker has a node');

    tracker.set('node', null);
    assert.notOk(tracker.get('node'), 'The tracker does not have a node');

    tracker = registry.getTracker(node1);
    assert.equal(
      tracker.get('node'),
      node1,
      'The node was re-attached to the tracker after calling getTracker again'
    );
  });

  test('Registry re-attaches destroyed resources to cached trackers', async function (assert) {
    const registry = this.subject();
    const id = 'some-id';

    const node1 = mockNode.create({ id });
    let tracker = registry.getTracker(node1);

    assert.ok(tracker.get('node'), 'The tracker has a node');

    node1.destroy();
    await settled();

    assert.ok(tracker.get('node').isDestroyed, 'The tracker node is destroyed');

    const node2 = mockNode.create({ id });
    tracker = registry.getTracker(node2);
    assert.equal(
      tracker.get('node'),
      node2,
      'Since node1 was destroyed but it matches the tracker of node2, node2 is attached to the tracker'
    );
  });

  test('Removes least recently used when something needs to be removed', function (assert) {
    const registry = this.subject();
    const activeNode = mockNode.create({ id: 'active' });
    const inactiveNode = mockNode.create({ id: 'inactive' });
    const limit = registry.get('registryRef').limit;

    // First put in the two tracked nodes
    registry.getTracker(activeNode);
    registry.getTracker(inactiveNode);

    for (let i = 0; i < limit; i++) {
      // Add a new tracker to the registry
      const newNode = mockNode.create({ id: `node-${i}` });
      registry.getTracker(newNode);

      // But read the active node tracker to keep it fresh
      registry.getTracker(activeNode);
    }

    const ref = registry.get('registryRef');
    assert.equal(ref.size, ref.limit, 'The limit was reached');

    assert.ok(
      ref.get('node:active'),
      'The active tracker is still in the registry despite being added first'
    );
    assert.notOk(
      ref.get('node:inactive'),
      'The inactive tracker got pushed out due to not being accessed'
    );
  });

  test('Trackers are created using the token authorizedRequest', function (assert) {
    const registry = this.subject();
    const node = mockNode.create();

    const tracker = registry.getTracker(node);

    tracker.get('poll').perform();
    assert.ok(
      this.tokenAuthorizedRequestSpy.calledWith(
        `/v1/client/stats?node_id=${node.get('id')}`
      ),
      'The token service authorizedRequest function was used'
    );

    return settled();
  });
});
