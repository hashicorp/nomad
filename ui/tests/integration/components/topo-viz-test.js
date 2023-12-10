/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { render, triggerEvent } from '@ember/test-helpers';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import { create } from 'ember-cli-page-object';
import { setupMirage } from 'ember-cli-mirage/test-support';
import sinon from 'sinon';
import faker from 'nomad-ui/mirage/faker';
import topoVizPageObject from 'nomad-ui/tests/pages/components/topo-viz';
import { HOSTS } from '../../../mirage/common';

const TopoViz = create(topoVizPageObject());

const alloc = (nodeId, jobId, taskGroupName, memory, cpu, props = {}) => ({
  id: faker.random.uuid(),
  taskGroupName,
  isScheduled: true,
  allocatedResources: {
    cpu,
    memory,
  },
  belongsTo: (type) => ({
    id: () => (type === 'job' ? jobId : nodeId),
  }),
  ...props,
});

const node = (datacenter, id, memory, cpu) => ({
  datacenter,
  id,
  name: `nomad@${HOSTS[Math.floor(Math.random() * 10) % HOSTS.length]}`,
  resources: { memory, cpu },
});

module('Integration | Component | TopoViz', function (hooks) {
  setupRenderingTest(hooks);
  setupMirage(hooks);

  const commonTemplate = hbs`
    <TopoViz
      @nodes={{this.nodes}}
      @allocations={{this.allocations}}
      @onAllocationSelect={{this.onAllocationSelect}}
      @onNodeSelect={{this.onNodeSelect}}
      @onDataError={{this.onDataError}} />
  `;

  test('presents as a FlexMasonry of datacenters', async function (assert) {
    assert.expect(6);

    this.setProperties({
      nodes: [node('dc1', 'node0', 1000, 500), node('dc2', 'node1', 1000, 500)],

      allocations: [
        alloc('node0', 'job1', 'group', 100, 100),
        alloc('node0', 'job1', 'group', 100, 100),
        alloc('node1', 'job1', 'group', 100, 100),
      ],
    });

    await render(commonTemplate);

    assert.equal(TopoViz.datacenters.length, 2);
    assert.equal(TopoViz.datacenters[0].nodes.length, 1);
    assert.equal(TopoViz.datacenters[1].nodes.length, 1);
    assert.equal(TopoViz.datacenters[0].nodes[0].memoryRects.length, 2);
    assert.equal(TopoViz.datacenters[1].nodes[0].memoryRects.length, 1);

    await componentA11yAudit(this.element, assert);
  });

  test('clicking on a node in a deeply nested TopoViz::Node will toggle node selection and call @onNodeSelect', async function (assert) {
    this.setProperties({
      // TopoViz must be dense for node selection to be a feature
      nodes: Array(55)
        .fill(null)
        .map((_, index) => node('dc1', `node${index}`, 1000, 500)),
      allocations: [],
      onNodeSelect: sinon.spy(),
    });

    await render(commonTemplate);

    await TopoViz.datacenters[0].nodes[0].selectNode();
    assert.ok(this.onNodeSelect.calledOnce);
    assert.equal(this.onNodeSelect.getCall(0).args[0].node, this.nodes[0]);

    await TopoViz.datacenters[0].nodes[0].selectNode();
    assert.ok(this.onNodeSelect.calledTwice);
    assert.equal(this.onNodeSelect.getCall(1).args[0], null);
  });

  test('clicking on an allocation in a deeply nested TopoViz::Node will update the topology object with selections and call @onAllocationSelect and @onNodeSelect', async function (assert) {
    this.setProperties({
      nodes: [node('dc1', 'node0', 1000, 500)],
      allocations: [alloc('node0', 'job1', 'group', 100, 100)],
      onNodeSelect: sinon.spy(),
      onAllocationSelect: sinon.spy(),
    });

    await render(commonTemplate);

    await TopoViz.datacenters[0].nodes[0].memoryRects[0].select();
    assert.ok(this.onAllocationSelect.calledOnce);
    assert.equal(
      this.onAllocationSelect.getCall(0).args[0],
      this.allocations[0]
    );
    assert.ok(this.onNodeSelect.calledOnce);

    await TopoViz.datacenters[0].nodes[0].memoryRects[0].select();
    assert.ok(this.onAllocationSelect.calledTwice);
    assert.equal(this.onAllocationSelect.getCall(1).args[0], null);
    assert.ok(this.onNodeSelect.calledTwice);
    assert.ok(this.onNodeSelect.alwaysCalledWith(null));
  });

  test('clicking on an allocation in a deeply nested TopoViz::Node will associate sibling allocations with curves', async function (assert) {
    this.setProperties({
      nodes: [
        node('dc1', 'node0', 1000, 500),
        node('dc1', 'node1', 1000, 500),
        node('dc1', 'node2', 1000, 500),
        node('dc2', 'node3', 1000, 500),
        node('dc2', 'node4', 1000, 500),
        node('dc2', 'node5', 1000, 500),
      ],
      allocations: [
        alloc('node0', 'job1', 'group', 100, 100),
        alloc('node0', 'job1', 'group', 100, 100),
        alloc('node1', 'job1', 'group', 100, 100),
        alloc('node2', 'job1', 'group', 100, 100),
        alloc('node0', 'job1', 'groupTwo', 100, 100),
        alloc('node1', 'job2', 'group', 100, 100),
        alloc('node2', 'job2', 'groupTwo', 100, 100),
      ],
      onNodeSelect: sinon.spy(),
      onAllocationSelect: sinon.spy(),
    });

    const selectedAllocations = this.allocations.filter(
      (alloc) =>
        alloc.belongsTo('job').id() === 'job1' &&
        alloc.taskGroupName === 'group'
    );

    await render(commonTemplate);

    assert.notOk(TopoViz.allocationAssociationsArePresent);

    await TopoViz.datacenters[0].nodes[0].memoryRects[0].select();

    assert.ok(TopoViz.allocationAssociationsArePresent);
    assert.equal(
      TopoViz.allocationAssociations.length,
      selectedAllocations.length * 2
    );

    // Lines get redrawn when the window resizes; make sure the lines persist.
    await triggerEvent(window, 'resize');
    assert.equal(
      TopoViz.allocationAssociations.length,
      selectedAllocations.length * 2
    );

    await TopoViz.datacenters[0].nodes[0].memoryRects[0].select();
    assert.notOk(TopoViz.allocationAssociationsArePresent);
  });

  test('when the count of sibling allocations is high enough relative to the node count, curves are not rendered', async function (assert) {
    this.setProperties({
      nodes: [node('dc1', 'node0', 1000, 500), node('dc1', 'node1', 1000, 500)],
      allocations: [
        // There need to be at least 10 sibling allocations to trigger this behavior
        alloc('node0', 'job1', 'group', 100, 100),
        alloc('node0', 'job1', 'group', 100, 100),
        alloc('node0', 'job1', 'group', 100, 100),
        alloc('node0', 'job1', 'group', 100, 100),
        alloc('node0', 'job1', 'group', 100, 100),
        alloc('node0', 'job1', 'group', 100, 100),
        alloc('node1', 'job1', 'group', 100, 100),
        alloc('node1', 'job1', 'group', 100, 100),
        alloc('node1', 'job1', 'group', 100, 100),
        alloc('node1', 'job1', 'group', 100, 100),
        alloc('node1', 'job1', 'group', 100, 100),
        alloc('node1', 'job1', 'group', 100, 100),
        alloc('node0', 'job1', 'groupTwo', 100, 100),
      ],
      onNodeSelect: sinon.spy(),
      onAllocationSelect: sinon.spy(),
    });

    await render(commonTemplate);
    assert.notOk(TopoViz.allocationAssociationsArePresent);

    await TopoViz.datacenters[0].nodes[0].memoryRects[0].select();
    assert.equal(TopoViz.allocationAssociations.length, 0);

    // Lines get redrawn when the window resizes; make sure that doesn't make the lines show up again
    await triggerEvent(window, 'resize');
    assert.equal(TopoViz.allocationAssociations.length, 0);
  });

  test('when one or more nodes are missing the resources property, those nodes are filtered out of the topology view and onDataError is called', async function (assert) {
    const badNode = node('dc1', 'node0', 1000, 500);
    delete badNode.resources;

    this.setProperties({
      nodes: [badNode, node('dc1', 'node1', 1000, 500)],
      allocations: [
        alloc('node0', 'job1', 'group', 100, 100),
        alloc('node0', 'job1', 'group', 100, 100),
        alloc('node1', 'job1', 'group', 100, 100),
        alloc('node1', 'job1', 'group', 100, 100),
        alloc('node0', 'job1', 'groupTwo', 100, 100),
      ],
      onNodeSelect: sinon.spy(),
      onAllocationSelect: sinon.spy(),
      onDataError: sinon.spy(),
    });

    await render(commonTemplate);

    assert.ok(this.onDataError.calledOnce);
    assert.deepEqual(this.onDataError.getCall(0).args[0], [
      {
        type: 'filtered-nodes',
        context: [this.nodes[0]],
      },
    ]);

    assert.equal(TopoViz.datacenters[0].nodes.length, 1);
  });
});
