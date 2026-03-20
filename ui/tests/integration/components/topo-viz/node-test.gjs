/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { create } from 'ember-cli-page-object';
import sinon from 'sinon';
import TopoVizNodeComponent from 'nomad-ui/components/topo-viz/node';
import faker from 'nomad-ui/mirage/faker';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import { setupMirage } from 'ember-cli-mirage/test-support';
import topoVizNodePageObject from 'nomad-ui/tests/pages/components/topo-viz/node';
import {
  formatScheduledBytes,
  formatScheduledHertz,
} from 'nomad-ui/utils/units';

const TopoVizNode = create(topoVizNodePageObject());

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

  test('presents as a div with a label and an svg with CPU and memory rows', async function (assert) {
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
      }),
    );

    await render(
      <template>
        <TopoVizNodeComponent
          @node={{this.node}}
          @isDense={{this.isDense}}
          @heightScale={{this.heightScale}}
          @onAllocationSelect={{this.onAllocationSelect}}
          @onAllocationFocus={{this.onAllocationFocus}}
          @onAllocationBlur={{this.onAllocationBlur}}
          @onNodeSelect={{this.onNodeSelect}}
        />
      </template>,
    );

    assert.ok(TopoVizNode.isPresent);
    assert.deepEqual(
      TopoVizNode.memoryRects.length,
      this.node.allocations.filterBy('allocation.isScheduled').length,
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
      }),
    );

    await render(
      <template>
        <TopoVizNodeComponent
          @node={{this.node}}
          @isDense={{this.isDense}}
          @heightScale={{this.heightScale}}
          @onAllocationSelect={{this.onAllocationSelect}}
          @onAllocationFocus={{this.onAllocationFocus}}
          @onAllocationBlur={{this.onAllocationBlur}}
          @onNodeSelect={{this.onNodeSelect}}
        />
      </template>,
    );

    assert.ok(TopoVizNode.label.includes(node.node.name));
    assert.ok(
      TopoVizNode.label.includes(
        `${this.node.allocations.filterBy('allocation.isScheduled').length} Allocs`,
      ),
    );
    assert.ok(
      TopoVizNode.label.includes(
        `${formatScheduledBytes(this.node.memory, 'MiB')}`,
      ),
    );
    assert.ok(
      TopoVizNode.label.includes(
        `${formatScheduledHertz(this.node.cpu, 'MHz')}`,
      ),
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
      }),
    );

    await render(
      <template>
        <TopoVizNodeComponent
          @node={{this.node}}
          @isDense={{this.isDense}}
          @heightScale={{this.heightScale}}
          @onAllocationSelect={{this.onAllocationSelect}}
          @onAllocationFocus={{this.onAllocationFocus}}
          @onAllocationBlur={{this.onAllocationBlur}}
          @onNodeSelect={{this.onNodeSelect}}
        />
      </template>,
    );

    assert.ok(TopoVizNode.statusIcon.includes('clock'));
    assert.deepEqual(TopoVizNode.statusIconLabel, 'Client is draining');
  });

  test('the status icon indicates when the node is ineligible for scheduling', async function (assert) {
    const node = nodeGen('Node One', 'dc1', 1000, 1000, { isEligible: false });
    this.setProperties(
      props({
        node: {
          ...node,
          allocations: [],
        },
      }),
    );

    await render(
      <template>
        <TopoVizNodeComponent
          @node={{this.node}}
          @isDense={{this.isDense}}
          @heightScale={{this.heightScale}}
          @onAllocationSelect={{this.onAllocationSelect}}
          @onAllocationFocus={{this.onAllocationFocus}}
          @onAllocationBlur={{this.onAllocationBlur}}
          @onNodeSelect={{this.onNodeSelect}}
        />
      </template>,
    );

    assert.ok(TopoVizNode.statusIcon.includes('lock'));
    assert.deepEqual(TopoVizNode.statusIconLabel, 'Client is ineligible');
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
      }),
    );

    await render(
      <template>
        <TopoVizNodeComponent
          @node={{this.node}}
          @isDense={{this.isDense}}
          @heightScale={{this.heightScale}}
          @onAllocationSelect={{this.onAllocationSelect}}
          @onAllocationFocus={{this.onAllocationFocus}}
          @onAllocationBlur={{this.onAllocationBlur}}
          @onNodeSelect={{this.onNodeSelect}}
        />
      </template>,
    );
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
      }),
    );

    await render(
      <template>
        <TopoVizNodeComponent
          @node={{this.node}}
          @isDense={{this.isDense}}
          @heightScale={{this.heightScale}}
          @onAllocationSelect={{this.onAllocationSelect}}
          @onAllocationFocus={{this.onAllocationFocus}}
          @onAllocationBlur={{this.onAllocationBlur}}
          @onNodeSelect={{this.onNodeSelect}}
        />
      </template>,
    );
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
      }),
    );

    await render(
      <template>
        <TopoVizNodeComponent
          @node={{this.node}}
          @isDense={{this.isDense}}
          @heightScale={{this.heightScale}}
          @onAllocationSelect={{this.onAllocationSelect}}
          @onAllocationFocus={{this.onAllocationFocus}}
          @onAllocationBlur={{this.onAllocationBlur}}
          @onNodeSelect={{this.onNodeSelect}}
        />
      </template>,
    );

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
      }),
    );

    await render(
      <template>
        <TopoVizNodeComponent
          @node={{this.node}}
          @isDense={{this.isDense}}
          @heightScale={{this.heightScale}}
          @onAllocationSelect={{this.onAllocationSelect}}
          @onAllocationFocus={{this.onAllocationFocus}}
          @onAllocationBlur={{this.onAllocationBlur}}
          @onNodeSelect={{this.onNodeSelect}}
        />
      </template>,
    );

    assert.ok(heightSpy.calledWith(this.node.memory));
    assert.dom('[data-test-memory-rect]').exists();
    assert.strictEqual(TopoVizNode.memoryRects[0].height, `${height}px`);
  });
});
