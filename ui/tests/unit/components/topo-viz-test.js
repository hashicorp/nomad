/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import setupGlimmerComponentFactory from 'nomad-ui/tests/helpers/glimmer-factory';

module('Unit | Component | TopoViz', function (hooks) {
  setupTest(hooks);
  setupGlimmerComponentFactory(hooks, 'topo-viz');

  test('the topology object properly organizes a tree of datacenters > nodes > allocations', async function (assert) {
    const nodes = [
      { datacenter: 'dc1', id: 'node0', resources: {} },
      { datacenter: 'dc2', id: 'node1', resources: {} },
      { datacenter: 'dc1', id: 'node2', resources: {} },
    ];

    const node0Allocs = [
      alloc({ nodeId: 'node0', jobId: 'job0', taskGroupName: 'group' }),
      alloc({ nodeId: 'node0', jobId: 'job1', taskGroupName: 'group' }),
    ];
    const node1Allocs = [
      alloc({ nodeId: 'node1', jobId: 'job0', taskGroupName: 'group' }),
      alloc({ nodeId: 'node1', jobId: 'job1', taskGroupName: 'group' }),
    ];
    const node2Allocs = [
      alloc({ nodeId: 'node2', jobId: 'job0', taskGroupName: 'group' }),
      alloc({ nodeId: 'node2', jobId: 'job1', taskGroupName: 'group' }),
    ];

    const allocations = [...node0Allocs, ...node1Allocs, ...node2Allocs];

    const topoViz = this.createComponent({ nodes, allocations });

    topoViz.buildTopology();

    assert.deepEqual(topoViz.topology.datacenters.mapBy('name'), [
      'dc1',
      'dc2',
    ]);
    assert.deepEqual(topoViz.topology.datacenters[0].nodes.mapBy('node'), [
      nodes[0],
      nodes[2],
    ]);
    assert.deepEqual(topoViz.topology.datacenters[1].nodes.mapBy('node'), [
      nodes[1],
    ]);
    assert.deepEqual(
      topoViz.topology.datacenters[0].nodes[0].allocations.mapBy('allocation'),
      node0Allocs
    );
    assert.deepEqual(
      topoViz.topology.datacenters[1].nodes[0].allocations.mapBy('allocation'),
      node1Allocs
    );
    assert.deepEqual(
      topoViz.topology.datacenters[0].nodes[1].allocations.mapBy('allocation'),
      node2Allocs
    );
  });

  test('the topology object contains an allocation index keyed by jobId+taskGroupName', async function (assert) {
    assert.expect(7);

    const allocations = [
      alloc({ nodeId: 'node0', jobId: 'job0', taskGroupName: 'one' }),
      alloc({ nodeId: 'node0', jobId: 'job0', taskGroupName: 'one' }),
      alloc({ nodeId: 'node0', jobId: 'job0', taskGroupName: 'two' }),
      alloc({ nodeId: 'node0', jobId: 'job1', taskGroupName: 'one' }),
      alloc({ nodeId: 'node0', jobId: 'job1', taskGroupName: 'two' }),
      alloc({ nodeId: 'node0', jobId: 'job1', taskGroupName: 'three' }),
      alloc({ nodeId: 'node0', jobId: 'job2', taskGroupName: 'one' }),
      alloc({ nodeId: 'node0', jobId: 'job2', taskGroupName: 'one' }),
      alloc({ nodeId: 'node0', jobId: 'job2', taskGroupName: 'one' }),
      alloc({ nodeId: 'node0', jobId: 'job2', taskGroupName: 'one' }),
    ];

    const nodes = [{ datacenter: 'dc1', id: 'node0', resources: {} }];
    const topoViz = this.createComponent({ nodes, allocations });

    topoViz.buildTopology();

    assert.deepEqual(
      Object.keys(topoViz.topology.allocationIndex).sort(),
      [
        JSON.stringify(['job0', 'one']),
        JSON.stringify(['job0', 'two']),

        JSON.stringify(['job1', 'one']),
        JSON.stringify(['job1', 'two']),
        JSON.stringify(['job1', 'three']),

        JSON.stringify(['job2', 'one']),
      ].sort()
    );

    Object.keys(topoViz.topology.allocationIndex).forEach((key) => {
      const [jobId, group] = JSON.parse(key);
      assert.deepEqual(
        topoViz.topology.allocationIndex[key].mapBy('allocation'),
        allocations.filter(
          (alloc) => alloc.jobId === jobId && alloc.taskGroupName === group
        )
      );
    });
  });

  test('isSingleColumn is true when there is only one datacenter', async function (assert) {
    const oneDc = [{ datacenter: 'dc1', id: 'node0', resources: {} }];
    const twoDc = [...oneDc, { datacenter: 'dc2', id: 'node1', resources: {} }];

    const topoViz1 = this.createComponent({ nodes: oneDc, allocations: [] });
    const topoViz2 = this.createComponent({ nodes: twoDc, allocations: [] });

    topoViz1.buildTopology();
    topoViz2.buildTopology();

    assert.ok(topoViz1.isSingleColumn);
    assert.notOk(topoViz2.isSingleColumn);
  });

  test('isSingleColumn is true when there are multiple datacenters with a high variance in node count', async function (assert) {
    const uniformDcs = [
      { datacenter: 'dc1', id: 'node0', resources: {} },
      { datacenter: 'dc2', id: 'node1', resources: {} },
    ];
    const skewedDcs = [
      { datacenter: 'dc1', id: 'node0', resources: {} },
      { datacenter: 'dc2', id: 'node1', resources: {} },
      { datacenter: 'dc2', id: 'node2', resources: {} },
      { datacenter: 'dc2', id: 'node3', resources: {} },
      { datacenter: 'dc2', id: 'node4', resources: {} },
    ];

    const twoColumnViz = this.createComponent({
      nodes: uniformDcs,
      allocations: [],
    });
    const oneColumViz = this.createComponent({
      nodes: skewedDcs,
      allocations: [],
    });

    twoColumnViz.buildTopology();
    oneColumViz.buildTopology();

    assert.notOk(twoColumnViz.isSingleColumn);
    assert.ok(oneColumViz.isSingleColumn);
  });

  test('datacenterIsSingleColumn is only ever false when isSingleColumn is false and the total node count is high', async function (assert) {
    const manyUniformNodes = Array(25)
      .fill(null)
      .map((_, index) => ({
        datacenter: index > 12 ? 'dc2' : 'dc1',
        id: `node${index}`,
        resources: {},
      }));
    const manySkewedNodes = Array(25)
      .fill(null)
      .map((_, index) => ({
        datacenter: index > 5 ? 'dc2' : 'dc1',
        id: `node${index}`,
        resources: {},
      }));

    const oneColumnViz = this.createComponent({
      nodes: manyUniformNodes,
      allocations: [],
    });
    const twoColumnViz = this.createComponent({
      nodes: manySkewedNodes,
      allocations: [],
    });

    oneColumnViz.buildTopology();
    twoColumnViz.buildTopology();

    assert.ok(oneColumnViz.datacenterIsSingleColumn);
    assert.notOk(oneColumnViz.isSingleColumn);

    assert.notOk(twoColumnViz.datacenterIsSingleColumn);
    assert.ok(twoColumnViz.isSingleColumn);
  });

  test('dataForAllocation correctly calculates proportion of node utilization and group key', async function (assert) {
    const nodes = [
      { datacenter: 'dc1', id: 'node0', resources: { cpu: 100, memory: 250 } },
    ];
    const allocations = [
      alloc({
        nodeId: 'node0',
        jobId: 'job0',
        taskGroupName: 'group',
        allocatedResources: { cpu: 50, memory: 25 },
      }),
    ];

    const topoViz = this.createComponent({ nodes, allocations });
    topoViz.buildTopology();

    assert.equal(
      topoViz.topology.datacenters[0].nodes[0].allocations[0].cpuPercent,
      0.5
    );
    assert.equal(
      topoViz.topology.datacenters[0].nodes[0].allocations[0].memoryPercent,
      0.1
    );
  });

  test('allocations that reference nonexistent nodes are ignored', async function (assert) {
    const nodes = [{ datacenter: 'dc1', id: 'node0', resources: {} }];

    const allocations = [
      alloc({ nodeId: 'node0', jobId: 'job0', taskGroupName: 'group' }),
      alloc({ nodeId: 'node404', jobId: 'job1', taskGroupName: 'group' }),
    ];

    const topoViz = this.createComponent({ nodes, allocations });

    topoViz.buildTopology();

    assert.deepEqual(topoViz.topology.datacenters[0].nodes.mapBy('node'), [
      nodes[0],
    ]);
    assert.deepEqual(
      topoViz.topology.datacenters[0].nodes[0].allocations.mapBy('allocation'),
      [allocations[0]]
    );
  });
});

function alloc(props) {
  return {
    ...props,
    allocatedResources: props.allocatedResources || {},
    belongsTo(type) {
      return {
        id() {
          return type === 'job' ? props.jobId : props.nodeId;
        },
      };
    },
  };
}
