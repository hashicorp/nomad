/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { findAll, render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { create } from 'ember-cli-page-object';
import sinon from 'sinon';
import faker from 'nomad-ui/mirage/faker';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import topoVisNodePageObject from 'nomad-ui/tests/pages/components/topo-viz/node';
import {
  formatScheduledBytes,
  formatScheduledHertz,
} from 'nomad-ui/utils/units';

const TopoVizNode = create(topoVisNodePageObject());

const nodeGen = (name, datacenter, memory, cpu, flags = {}) => ({
  datacenter,
  memory,
  cpu,
  isSelected: !!flags.isSelected,
  node: {
    name,
    isEligible: flags.isEligible || flags.isEligible == null,
    isDraining: !!flags.isDraining,
  },
});

const allocGen = (node, memory, cpu, isScheduled = true) => ({
  memory,
  cpu,
  isSelected: false,
  memoryPercent: memory / node.memory,
  cpuPercent: cpu / node.cpu,
  allocation: {
    id: faker.random.uuid(),
    isScheduled,
  },
});

const props = (overrides) => ({
  isDense: false,
  heightScale: () => 50,
  onAllocationSelect: sinon.spy(),
  onNodeSelect: sinon.spy(),
  onAllocationFocus: sinon.spy(),
  onAllocationBlur: sinon.spy(),
  ...overrides,
});

module('Integration | Component | TopoViz::Node', function (hooks) {
  setupRenderingTest(hooks);
  setupMirage(hooks);

  const commonTemplate = hbs`
    <TopoViz::Node
      @node={{this.node}}
      @isDense={{this.isDense}}
      @heightScale={{this.heightScale}}
      @onAllocationSelect={{this.onAllocationSelect}}
      @onAllocationFocus={{this.onAllocationFocus}}
      @onAllocationBlur={{this.onAllocationBlur}}
      @onNodeSelect={{this.onNodeSelect}} />
  `;

  test('presents as a div with a label and an svg with CPU and memory rows', async function (assert) {
    assert.expect(4);

    const node = nodeGen('Node One', 'dc1', 1000, 1000);
    this.setProperties(
      props({
        node: {
          ...node,
          allocations: [
            allocGen(node, 100, 100),
            allocGen(node, 250, 250),
            allocGen(node, 300, 300, false),
          ],
        },
      })
    );

    await render(commonTemplate);

    assert.ok(TopoVizNode.isPresent);
    assert.equal(
      TopoVizNode.memoryRects.length,
      this.node.allocations.filterBy('allocation.isScheduled').length
    );
    assert.ok(TopoVizNode.cpuRects.length);

    await componentA11yAudit(this.element, assert);
  });

  test('the label contains aggregate information about the node', async function (assert) {
    const node = nodeGen('Node One', 'dc1', 1000, 1000);
    this.setProperties(
      props({
        node: {
          ...node,
          allocations: [
            allocGen(node, 100, 100),
            allocGen(node, 250, 250),
            allocGen(node, 300, 300, false),
          ],
        },
      })
    );

    await render(commonTemplate);

    assert.ok(TopoVizNode.label.includes(node.node.name));
    assert.ok(
      TopoVizNode.label.includes(
        `${
          this.node.allocations.filterBy('allocation.isScheduled').length
        } Allocs`
      )
    );
    assert.ok(
      TopoVizNode.label.includes(
        `${formatScheduledBytes(this.node.memory, 'MiB')}`
      )
    );
    assert.ok(
      TopoVizNode.label.includes(
        `${formatScheduledHertz(this.node.cpu, 'MHz')}`
      )
    );
  });

  test('the status icon indicates when the node is draining', async function (assert) {
    const node = nodeGen('Node One', 'dc1', 1000, 1000, { isDraining: true });
    this.setProperties(
      props({
        node: {
          ...node,
          allocations: [],
        },
      })
    );

    await render(commonTemplate);

    assert.ok(TopoVizNode.statusIcon.includes('icon-is-clock-outline'));
    assert.equal(TopoVizNode.statusIconLabel, 'Client is draining');
  });

  test('the status icon indicates when the node is ineligible for scheduling', async function (assert) {
    const node = nodeGen('Node One', 'dc1', 1000, 1000, { isEligible: false });
    this.setProperties(
      props({
        node: {
          ...node,
          allocations: [],
        },
      })
    );

    await render(commonTemplate);

    assert.ok(TopoVizNode.statusIcon.includes('icon-is-lock-closed'));
    assert.equal(TopoVizNode.statusIconLabel, 'Client is ineligible');
  });

  test('when isDense is false, clicking the node does nothing', async function (assert) {
    const node = nodeGen('Node One', 'dc1', 1000, 1000);
    this.setProperties(
      props({
        isDense: false,
        node: {
          ...node,
          allocations: [],
        },
      })
    );

    await render(commonTemplate);
    await TopoVizNode.selectNode();

    assert.notOk(TopoVizNode.nodeIsInteractive);
    assert.notOk(this.onNodeSelect.called);
  });

  test('when isDense is true, clicking the node calls onNodeSelect', async function (assert) {
    const node = nodeGen('Node One', 'dc1', 1000, 1000);
    this.setProperties(
      props({
        isDense: true,
        node: {
          ...node,
          allocations: [],
        },
      })
    );

    await render(commonTemplate);
    await TopoVizNode.selectNode();

    assert.ok(TopoVizNode.nodeIsInteractive);
    assert.ok(this.onNodeSelect.called);
    assert.ok(this.onNodeSelect.calledWith(this.node));
  });

  test('the node gets the is-selected class when the node is selected', async function (assert) {
    const node = nodeGen('Node One', 'dc1', 1000, 1000, { isSelected: true });
    this.setProperties(
      props({
        isDense: true,
        node: {
          ...node,
          allocations: [],
        },
      })
    );

    await render(commonTemplate);

    assert.ok(TopoVizNode.nodeIsSelected);
  });

  test('the node gets its height form the @heightScale arg', async function (assert) {
    const node = nodeGen('Node One', 'dc1', 1000, 1000);
    const height = 50;
    const heightSpy = sinon.spy();
    this.setProperties(
      props({
        heightScale: (...args) => {
          heightSpy(...args);
          return height;
        },
        node: {
          ...node,
          allocations: [allocGen(node, 100, 100), allocGen(node, 250, 250)],
        },
      })
    );

    await render(commonTemplate);

    assert.ok(heightSpy.called);
    assert.ok(heightSpy.calledWith(this.node.memory));
    assert.equal(TopoVizNode.memoryRects[0].height, `${height}px`);
  });

  test('each allocation gets a memory rect and a cpu rect', async function (assert) {
    const node = nodeGen('Node One', 'dc1', 1000, 1000);
    this.setProperties(
      props({
        node: {
          ...node,
          allocations: [allocGen(node, 100, 100), allocGen(node, 250, 250)],
        },
      })
    );

    await render(commonTemplate);

    assert.equal(TopoVizNode.memoryRects.length, this.node.allocations.length);
    assert.equal(TopoVizNode.cpuRects.length, this.node.allocations.length);
  });

  test('each allocation is sized according to its percentage of utilization', async function (assert) {
    assert.expect(4);

    const node = nodeGen('Node One', 'dc1', 1000, 1000);
    this.setProperties(
      props({
        node: {
          ...node,
          allocations: [allocGen(node, 100, 100), allocGen(node, 250, 250)],
        },
      })
    );

    await render(hbs`
      <div style="width:100px">
        <TopoViz::Node
          @node={{this.node}}
          @isDense={{this.isDense}}
          @heightScale={{this.heightScale}}
          @onAllocationSelect={{this.onAllocationSelect}}
          @onNodeSelect={{this.onNodeSelect}} />
      </div>
    `);

    // Remove the width of the padding and the label from the SVG width
    const width = 100 - 5 - 5 - 20;
    this.node.allocations.forEach((alloc, index) => {
      const memWidth = alloc.memoryPercent * width - (index === 0 ? 0.5 : 1);
      const cpuWidth = alloc.cpuPercent * width - (index === 0 ? 0.5 : 1);
      assert.equal(TopoVizNode.memoryRects[index].width, `${memWidth}px`);
      assert.equal(TopoVizNode.cpuRects[index].width, `${cpuWidth}px`);
    });
  });

  test('clicking either the memory or cpu rect for an allocation will call onAllocationSelect', async function (assert) {
    const node = nodeGen('Node One', 'dc1', 1000, 1000);
    this.setProperties(
      props({
        node: {
          ...node,
          allocations: [allocGen(node, 100, 100), allocGen(node, 250, 250)],
        },
      })
    );

    await render(commonTemplate);

    await TopoVizNode.memoryRects[0].select();
    assert.equal(this.onAllocationSelect.callCount, 1);
    assert.ok(this.onAllocationSelect.calledWith(this.node.allocations[0]));

    await TopoVizNode.cpuRects[0].select();
    assert.equal(this.onAllocationSelect.callCount, 2);

    await TopoVizNode.cpuRects[1].select();
    assert.equal(this.onAllocationSelect.callCount, 3);
    assert.ok(this.onAllocationSelect.calledWith(this.node.allocations[1]));

    await TopoVizNode.memoryRects[1].select();
    assert.equal(this.onAllocationSelect.callCount, 4);
  });

  test('hovering over a memory or cpu rect for an allocation will call onAllocationFocus', async function (assert) {
    const node = nodeGen('Node One', 'dc1', 1000, 1000);
    this.setProperties(
      props({
        node: {
          ...node,
          allocations: [allocGen(node, 100, 100), allocGen(node, 250, 250)],
        },
      })
    );

    await render(commonTemplate);

    await TopoVizNode.memoryRects[0].hover();
    assert.equal(this.onAllocationFocus.callCount, 1);
    assert.equal(
      this.onAllocationFocus.getCall(0).args[0].allocation,
      this.node.allocations[0].allocation
    );
    assert.equal(
      this.onAllocationFocus.getCall(0).args[1],
      findAll('[data-test-memory-rect]')[0]
    );

    await TopoVizNode.cpuRects[1].hover();
    assert.equal(this.onAllocationFocus.callCount, 2);
    assert.equal(
      this.onAllocationFocus.getCall(1).args[0].allocation,
      this.node.allocations[1].allocation
    );
    assert.equal(
      this.onAllocationFocus.getCall(1).args[1],
      findAll('[data-test-cpu-rect]')[1]
    );
  });

  test('leaving the entire node will call onAllocationBlur, which allows for the tooltip transitions', async function (assert) {
    const node = nodeGen('Node One', 'dc1', 1000, 1000);
    this.setProperties(
      props({
        node: {
          ...node,
          allocations: [allocGen(node, 100, 100), allocGen(node, 250, 250)],
        },
      })
    );

    await render(commonTemplate);

    await TopoVizNode.memoryRects[0].hover();
    assert.equal(this.onAllocationFocus.callCount, 1);
    assert.equal(this.onAllocationBlur.callCount, 0);

    await TopoVizNode.memoryRects[0].mouseleave();
    assert.equal(this.onAllocationBlur.callCount, 0);

    await TopoVizNode.mouseout();
    assert.equal(this.onAllocationBlur.callCount, 1);
  });

  test('allocations are sorted by smallest to largest delta of memory to cpu percent utilizations', async function (assert) {
    assert.expect(10);

    const node = nodeGen('Node One', 'dc1', 1000, 1000);

    const evenAlloc = allocGen(node, 100, 100);
    const mediumMemoryAlloc = allocGen(node, 200, 150);
    const largeMemoryAlloc = allocGen(node, 300, 50);
    const mediumCPUAlloc = allocGen(node, 150, 200);
    const largeCPUAlloc = allocGen(node, 50, 300);

    this.setProperties(
      props({
        node: {
          ...node,
          allocations: [
            largeCPUAlloc,
            mediumCPUAlloc,
            evenAlloc,
            mediumMemoryAlloc,
            largeMemoryAlloc,
          ],
        },
      })
    );

    await render(commonTemplate);

    const expectedOrder = [
      evenAlloc,
      mediumCPUAlloc,
      mediumMemoryAlloc,
      largeCPUAlloc,
      largeMemoryAlloc,
    ];
    expectedOrder.forEach((alloc, index) => {
      assert.equal(TopoVizNode.memoryRects[index].id, alloc.allocation.id);
      assert.equal(TopoVizNode.cpuRects[index].id, alloc.allocation.id);
    });
  });

  test('when there are no allocations, a "no allocations" note is shown', async function (assert) {
    const node = nodeGen('Node One', 'dc1', 1000, 1000);
    this.setProperties(
      props({
        node: {
          ...node,
          allocations: [],
        },
      })
    );

    await render(commonTemplate);
    assert.equal(TopoVizNode.emptyMessage, 'Empty Client');
  });
});
