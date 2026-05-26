/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { find, render, triggerEvent } from '@ember/test-helpers';
import { setupRenderingTest } from 'ember-qunit';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import { create } from 'ember-cli-page-object';
import { setupMirage } from 'ember-cli-mirage/test-support';
import sinon from 'sinon';
import faker from 'nomad-ui/mirage/faker';
import topoVizPageObject from 'nomad-ui/tests/pages/components/topo-viz';
import { HOSTS } from '../../../mirage/common';
import TopoVizComponent from 'nomad-ui/components/topo-viz';

const TopoViz = create(topoVizPageObject());
const noop = () => {};

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

  test('presents as a FlexMasonry of datacenters', async function (assert) {
    const nodes = [
      node('dc1', 'node0', 1000, 500),
      node('dc2', 'node1', 1000, 500),
    ];

    const allocations = [
      alloc('node0', 'job1', 'group', 100, 100),
      alloc('node0', 'job1', 'group', 100, 100),
      alloc('node1', 'job1', 'group', 100, 100),
    ];

    await render(
      <template>
        <TopoVizComponent
          @nodes={{nodes}}
          @allocations={{allocations}}
          @onAllocationSelect={{noop}}
          @onNodeSelect={{noop}}
          @onDataError={{noop}}
        />
      </template>,
    );

    assert.deepEqual(TopoViz.datacenters.length, 2);
    assert.deepEqual(TopoViz.datacenters[0].nodes.length, 1);
    assert.deepEqual(TopoViz.datacenters[1].nodes.length, 1);
    assert.deepEqual(TopoViz.datacenters[0].nodes[0].memoryRects.length, 2);
    assert.deepEqual(TopoViz.datacenters[1].nodes[0].memoryRects.length, 1);

    await componentA11yAudit(find('[data-test-topo-viz]'), assert);
  });

  test('clicking on a node in a deeply nested TopoViz::Node will toggle node selection and call @onNodeSelect', async function (assert) {
    // TopoViz must be dense for node selection to be a feature
    const nodes = Array(55)
      .fill(null)
      .map((_, index) => node('dc1', `node${index}`, 1000, 500));
    const allocations = [];
    const onNodeSelect = sinon.spy();

    await render(
      <template>
        <TopoVizComponent
          @nodes={{nodes}}
          @allocations={{allocations}}
          @onAllocationSelect={{noop}}
          @onNodeSelect={{onNodeSelect}}
          @onDataError={{noop}}
        />
      </template>,
    );

    await TopoViz.datacenters[0].nodes[0].selectNode();
    assert.ok(onNodeSelect.calledOnce);
    assert.deepEqual(onNodeSelect.getCall(0).args[0].node, nodes[0]);

    await TopoViz.datacenters[0].nodes[0].selectNode();
    assert.ok(onNodeSelect.calledTwice);
    assert.deepEqual(onNodeSelect.getCall(1).args[0], null);
  });

  test('clicking on an allocation in a deeply nested TopoViz::Node will update the topology object with selections and call @onAllocationSelect and @onNodeSelect', async function (assert) {
    const nodes = [node('dc1', 'node0', 1000, 500)];
    const allocations = [alloc('node0', 'job1', 'group', 100, 100)];
    const onNodeSelect = sinon.spy();
    const onAllocationSelect = sinon.spy();

    await render(
      <template>
        <TopoVizComponent
          @nodes={{nodes}}
          @allocations={{allocations}}
          @onAllocationSelect={{onAllocationSelect}}
          @onNodeSelect={{onNodeSelect}}
          @onDataError={{noop}}
        />
      </template>,
    );

    await TopoViz.datacenters[0].nodes[0].memoryRects[0].select();
    assert.ok(onAllocationSelect.calledOnce);
    assert.deepEqual(onAllocationSelect.getCall(0).args[0], allocations[0]);
    assert.ok(onNodeSelect.calledOnce);

    await TopoViz.datacenters[0].nodes[0].memoryRects[0].select();
    assert.ok(onAllocationSelect.calledTwice);
    assert.deepEqual(onAllocationSelect.getCall(1).args[0], null);
    assert.ok(onNodeSelect.calledTwice);
    assert.ok(onNodeSelect.alwaysCalledWith(null));
  });

  test('clicking on an allocation in a deeply nested TopoViz::Node will associate sibling allocations with curves', async function (assert) {
    const nodes = [
      node('dc1', 'node0', 1000, 500),
      node('dc1', 'node1', 1000, 500),
      node('dc1', 'node2', 1000, 500),
      node('dc2', 'node3', 1000, 500),
      node('dc2', 'node4', 1000, 500),
      node('dc2', 'node5', 1000, 500),
    ];
    const allocations = [
      alloc('node0', 'job1', 'group', 100, 100),
      alloc('node0', 'job1', 'group', 100, 100),
      alloc('node1', 'job1', 'group', 100, 100),
      alloc('node2', 'job1', 'group', 100, 100),
      alloc('node0', 'job1', 'groupTwo', 100, 100),
      alloc('node1', 'job2', 'group', 100, 100),
      alloc('node2', 'job2', 'groupTwo', 100, 100),
    ];
    const onNodeSelect = sinon.spy();
    const onAllocationSelect = sinon.spy();

    const selectedAllocations = allocations.filter(
      (allocEntry) =>
        allocEntry.belongsTo('job').id() === 'job1' &&
        allocEntry.taskGroupName === 'group',
    );

    await render(
      <template>
        <TopoVizComponent
          @nodes={{nodes}}
          @allocations={{allocations}}
          @onAllocationSelect={{onAllocationSelect}}
          @onNodeSelect={{onNodeSelect}}
          @onDataError={{noop}}
        />
      </template>,
    );

    assert.notOk(TopoViz.allocationAssociationsArePresent);

    await TopoViz.datacenters[0].nodes[0].memoryRects[0].select();

    assert.ok(TopoViz.allocationAssociationsArePresent);
    assert.deepEqual(
      TopoViz.allocationAssociations.length,
      selectedAllocations.length * 2,
    );

    // Lines get redrawn when the window resizes; make sure the lines persist.
    await triggerEvent(window, 'resize');
    assert.deepEqual(
      TopoViz.allocationAssociations.length,
      selectedAllocations.length * 2,
    );

    await TopoViz.datacenters[0].nodes[0].memoryRects[0].select();
    assert.notOk(TopoViz.allocationAssociationsArePresent);
  });

  test('when the count of sibling allocations is high enough relative to the node count, curves are not rendered', async function (assert) {
    const nodes = [
      node('dc1', 'node0', 1000, 500),
      node('dc1', 'node1', 1000, 500),
    ];
    const allocations = [
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
    ];
    const onNodeSelect = sinon.spy();
    const onAllocationSelect = sinon.spy();

    await render(
      <template>
        <TopoVizComponent
          @nodes={{nodes}}
          @allocations={{allocations}}
          @onAllocationSelect={{onAllocationSelect}}
          @onNodeSelect={{onNodeSelect}}
          @onDataError={{noop}}
        />
      </template>,
    );
    assert.notOk(TopoViz.allocationAssociationsArePresent);

    await TopoViz.datacenters[0].nodes[0].memoryRects[0].select();
    assert.deepEqual(TopoViz.allocationAssociations.length, 0);

    // Lines get redrawn when the window resizes; make sure that doesn't make the lines show up again
    await triggerEvent(window, 'resize');
    assert.deepEqual(TopoViz.allocationAssociations.length, 0);
  });

  test('when one or more nodes are missing the resources property, those nodes are filtered out of the topology view and onDataError is called', async function (assert) {
    const badNode = node('dc1', 'node0', 1000, 500);
    delete badNode.resources;

    const nodes = [badNode, node('dc1', 'node1', 1000, 500)];
    const allocations = [
      alloc('node0', 'job1', 'group', 100, 100),
      alloc('node0', 'job1', 'group', 100, 100),
      alloc('node1', 'job1', 'group', 100, 100),
      alloc('node1', 'job1', 'group', 100, 100),
      alloc('node0', 'job1', 'groupTwo', 100, 100),
    ];
    const onNodeSelect = sinon.spy();
    const onAllocationSelect = sinon.spy();
    const onDataError = sinon.spy();

    await render(
      <template>
        <TopoVizComponent
          @nodes={{nodes}}
          @allocations={{allocations}}
          @onAllocationSelect={{onAllocationSelect}}
          @onNodeSelect={{onNodeSelect}}
          @onDataError={{onDataError}}
        />
      </template>,
    );

    assert.ok(onDataError.calledOnce);
    assert.deepEqual(onDataError.getCall(0).args[0], [
      {
        type: 'filtered-nodes',
        context: [nodes[0]],
      },
    ]);

    assert.deepEqual(TopoViz.datacenters[0].nodes.length, 1);
  });
});
