/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import jobClientStatus from 'nomad-ui/utils/properties/job-client-status';
import EmberObject from '@ember/object';

class JobClientStatusMock extends EmberObject {
  constructor(job, nodes) {
    super(...arguments);
    this.job = job;
    this.nodes = nodes;
  }

  @jobClientStatus('nodes', 'job') jobClientStatus;

  get(key) {
    switch (key) {
      case 'job':
        return this.job;
      case 'nodes':
        return this.nodes;
    }
  }
}

class NodeMock {
  constructor(id, datacenter) {
    this.id = id;
    this.datacenter = datacenter;
  }

  get(key) {
    switch (key) {
      case 'id':
        return this.id;
    }
  }
}

class AllocationMock {
  constructor(node, clientStatus) {
    this.node = node;
    this.clientStatus = clientStatus;
  }

  belongsTo() {
    const self = this;
    return {
      id() {
        return self.node.id;
      },
    };
  }
}

module('Unit | Util | JobClientStatus', function () {
  test('it handles the case where all nodes are running', async function (assert) {
    const node = new NodeMock('node-1', 'dc1');
    const nodes = [node];
    const job = {
      datacenters: ['dc1'],
      status: 'running',
      allocations: [new AllocationMock(node, 'running')],
      taskGroups: [{}],
    };
    const expected = {
      byNode: {
        'node-1': 'running',
      },
      byStatus: {
        running: ['node-1'],
        complete: [],
        degraded: [],
        failed: [],
        lost: [],
        notScheduled: [],
        queued: [],
        starting: [],
        unknown: [],
      },
      totalNodes: 1,
    };

    const mock = new JobClientStatusMock(job, nodes);
    let result = mock.jobClientStatus;

    assert.deepEqual(result, expected);
  });

  test('it handles the degraded case where a node has a failing allocation', async function (assert) {
    const node = new NodeMock('node-2', 'dc1');
    const nodes = [node];
    const job = {
      datacenters: ['dc1'],
      status: 'running',
      allocations: [
        new AllocationMock(node, 'running'),
        new AllocationMock(node, 'failed'),
        new AllocationMock(node, 'running'),
      ],
      taskGroups: [{}, {}, {}],
    };
    const expected = {
      byNode: {
        'node-2': 'degraded',
      },
      byStatus: {
        running: [],
        complete: [],
        degraded: ['node-2'],
        failed: [],
        lost: [],
        notScheduled: [],
        queued: [],
        starting: [],
        unknown: [],
      },
      totalNodes: 1,
    };

    const mock = new JobClientStatusMock(job, nodes);
    let result = mock.jobClientStatus;

    assert.deepEqual(result, expected);
  });

  test('it handles the case where a node has all lost allocations', async function (assert) {
    const node = new NodeMock('node-1', 'dc1');
    const nodes = [node];
    const job = {
      datacenters: ['dc1'],
      status: 'running',
      allocations: [
        new AllocationMock(node, 'lost'),
        new AllocationMock(node, 'lost'),
        new AllocationMock(node, 'lost'),
      ],
      taskGroups: [{}, {}, {}],
    };
    const expected = {
      byNode: {
        'node-1': 'lost',
      },
      byStatus: {
        running: [],
        complete: [],
        degraded: [],
        failed: [],
        lost: ['node-1'],
        notScheduled: [],
        queued: [],
        starting: [],
        unknown: [],
      },
      totalNodes: 1,
    };

    const mock = new JobClientStatusMock(job, nodes);
    let result = mock.jobClientStatus;

    assert.deepEqual(result, expected);
  });

  test('it handles the case where a node has all failed allocations', async function (assert) {
    const node = new NodeMock('node-1', 'dc1');
    const nodes = [node];
    const job = {
      datacenters: ['dc1'],
      status: 'running',
      allocations: [
        new AllocationMock(node, 'failed'),
        new AllocationMock(node, 'failed'),
        new AllocationMock(node, 'failed'),
      ],
      taskGroups: [{}, {}, {}],
    };
    const expected = {
      byNode: {
        'node-1': 'failed',
      },
      byStatus: {
        running: [],
        complete: [],
        degraded: [],
        failed: ['node-1'],
        lost: [],
        notScheduled: [],
        queued: [],
        starting: [],
        unknown: [],
      },
      totalNodes: 1,
    };

    const mock = new JobClientStatusMock(job, nodes);
    let result = mock.jobClientStatus;

    assert.deepEqual(result, expected);
  });

  test('it handles the degraded case where the expected number of allocations doesnt match the actual number of allocations', async function (assert) {
    const node = new NodeMock('node-1', 'dc1');
    const nodes = [node];
    const job = {
      datacenters: ['dc1'],
      status: 'running',
      allocations: [
        new AllocationMock(node, 'running'),
        new AllocationMock(node, 'running'),
        new AllocationMock(node, 'running'),
      ],
      taskGroups: [{}, {}, {}, {}],
    };
    const expected = {
      byNode: {
        'node-1': 'degraded',
      },
      byStatus: {
        running: [],
        complete: [],
        degraded: ['node-1'],
        failed: [],
        lost: [],
        notScheduled: [],
        queued: [],
        starting: [],
        unknown: [],
      },
      totalNodes: 1,
    };

    const mock = new JobClientStatusMock(job, nodes);
    let result = mock.jobClientStatus;

    assert.deepEqual(result, expected);
  });

  test('it handles the not scheduled case where a node has no allocations', async function (assert) {
    const node = new NodeMock('node-1', 'dc1');
    const nodes = [node];
    const job = {
      datacenters: ['dc1'],
      status: 'running',
      allocations: [],
      taskGroups: [],
    };
    const expected = {
      byNode: {
        'node-1': 'notScheduled',
      },
      byStatus: {
        running: [],
        complete: [],
        degraded: [],
        failed: [],
        lost: [],
        notScheduled: ['node-1'],
        queued: [],
        starting: [],
        unknown: [],
      },
      totalNodes: 1,
    };

    const mock = new JobClientStatusMock(job, nodes);
    let result = mock.jobClientStatus;

    assert.deepEqual(result, expected);
  });

  test('it handles the queued case where the job is pending', async function (assert) {
    const node = new NodeMock('node-1', 'dc1');
    const nodes = [node];
    const job = {
      datacenters: ['dc1'],
      status: 'pending',
      allocations: [
        new AllocationMock(node, 'starting'),
        new AllocationMock(node, 'starting'),
        new AllocationMock(node, 'starting'),
      ],
      taskGroups: [{}, {}, {}, {}],
    };
    const expected = {
      byNode: {
        'node-1': 'queued',
      },
      byStatus: {
        running: [],
        complete: [],
        degraded: [],
        failed: [],
        lost: [],
        notScheduled: [],
        queued: ['node-1'],
        starting: [],
        unknown: [],
      },
      totalNodes: 1,
    };

    const mock = new JobClientStatusMock(job, nodes);
    let result = mock.jobClientStatus;

    assert.deepEqual(result, expected);
  });

  test('it filters nodes by the datacenter of the job', async function (assert) {
    const node1 = new NodeMock('node-1', 'dc1');
    const node2 = new NodeMock('node-2', 'dc2');
    const nodes = [node1, node2];
    const job = {
      datacenters: ['dc1'],
      status: 'running',
      allocations: [
        new AllocationMock(node1, 'running'),
        new AllocationMock(node2, 'failed'),
        new AllocationMock(node1, 'running'),
      ],
      taskGroups: [{}, {}],
    };
    const expected = {
      byNode: {
        'node-1': 'running',
      },
      byStatus: {
        running: ['node-1'],
        complete: [],
        degraded: [],
        failed: [],
        lost: [],
        notScheduled: [],
        queued: [],
        starting: [],
        unknown: [],
      },
      totalNodes: 1,
    };

    const mock = new JobClientStatusMock(job, nodes);
    let result = mock.jobClientStatus;

    assert.deepEqual(result, expected);
  });
});
