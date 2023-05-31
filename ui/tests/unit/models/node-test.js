/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

module('Unit | Model | NodePool', function (hooks) {
  setupTest(hooks);

  test('a node pool can be associated with multiples nodes', function (assert) {
    const store = this.owner.lookup('service:store');
    this.subject = () => store.createRecord('node-pool');
    const nodePool = this.subject();
    const node1 = store.createRecord('node', { name: 'Node 1' });
    const node2 = store.createRecord('node', { name: 'Node 2' });

    nodePool.get('nodes').pushObject(node1);
    nodePool.get('nodes').pushObject(node2);

    const nodes = nodePool.get('nodes');

    assert.equal(nodes.length, 2);
    assert.ok(nodes.includes(node1));
    assert.ok(nodes.includes(node2));
  });
});
