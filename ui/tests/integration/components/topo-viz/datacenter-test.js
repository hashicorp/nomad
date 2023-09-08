/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { find, render } from '@ember/test-helpers';
import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { setupMirage } from 'ember-cli-mirage/test-support';
import { componentA11yAudit } from 'nomad-ui/tests/helpers/a11y-audit';
import { create } from 'ember-cli-page-object';
import sinon from 'sinon';
import faker from 'nomad-ui/mirage/faker';
import topoVizDatacenterPageObject from 'nomad-ui/tests/pages/components/topo-viz/datacenter';
import { formatBytes, formatHertz } from 'nomad-ui/utils/units';

const TopoVizDatacenter = create(topoVizDatacenterPageObject());

const nodeGen = (name, datacenter, memory, cpu, allocations = []) => ({
  datacenter,
  memory,
  cpu,
  node: { name },
  allocations: allocations.map((alloc) => ({
    memory: alloc.memory,
    cpu: alloc.cpu,
    memoryPercent: alloc.memory / memory,
    cpuPercent: alloc.cpu / cpu,
    allocation: {
      id: faker.random.uuid(),
      isScheduled: true,
    },
  })),
});

// Used in Array#reduce to sum by a property common to an array of objects
const sumBy = (prop) => (sum, obj) => (sum += obj[prop]);

module('Integration | Component | TopoViz::Datacenter', function (hooks) {
  setupRenderingTest(hooks);
  setupMirage(hooks);

  const commonProps = (props) => ({
    isSingleColumn: true,
    isDense: false,
    heightScale: () => 50,
    onAllocationSelect: sinon.spy(),
    onNodeSelect: sinon.spy(),
    ...props,
  });

  const commonTemplate = hbs`
    <TopoViz::Datacenter
      @datacenter={{this.datacenter}}
      @isSingleColumn={{this.isSingleColumn}}
      @isDense={{this.isDense}}
      @heightScale={{this.heightScale}}
      @onAllocationSelect={{this.onAllocationSelect}}
      @onNodeSelect={{this.onNodeSelect}} />
  `;

  test('presents as a div with a label and a FlexMasonry with a collection of nodes', async function (assert) {
    assert.expect(3);

    this.setProperties(
      commonProps({
        datacenter: {
          name: 'dc1',
          nodes: [nodeGen('node-1', 'dc1', 1000, 500)],
        },
      })
    );

    await render(commonTemplate);

    assert.ok(TopoVizDatacenter.isPresent);
    assert.equal(TopoVizDatacenter.nodes.length, this.datacenter.nodes.length);

    await componentA11yAudit(this.element, assert);
  });

  test('datacenter stats are an aggregate of node stats', async function (assert) {
    this.setProperties(
      commonProps({
        datacenter: {
          name: 'dc1',
          nodes: [
            nodeGen('node-1', 'dc1', 1000, 500, [
              { memory: 100, cpu: 300 },
              { memory: 200, cpu: 50 },
            ]),
            nodeGen('node-2', 'dc1', 1500, 100, [
              { memory: 50, cpu: 80 },
              { memory: 100, cpu: 20 },
            ]),
            nodeGen('node-3', 'dc1', 2000, 300),
            nodeGen('node-4', 'dc1', 3000, 200),
          ],
        },
      })
    );

    await render(commonTemplate);

    const allocs = this.datacenter.nodes.reduce(
      (allocs, node) => allocs.concat(node.allocations),
      []
    );
    const memoryReserved = allocs.reduce(sumBy('memory'), 0);
    const cpuReserved = allocs.reduce(sumBy('cpu'), 0);
    const memoryTotal = this.datacenter.nodes.reduce(sumBy('memory'), 0);
    const cpuTotal = this.datacenter.nodes.reduce(sumBy('cpu'), 0);

    assert.ok(TopoVizDatacenter.label.includes(this.datacenter.name));
    assert.ok(
      TopoVizDatacenter.label.includes(`${this.datacenter.nodes.length} Nodes`)
    );
    assert.ok(TopoVizDatacenter.label.includes(`${allocs.length} Allocs`));
    assert.ok(
      TopoVizDatacenter.label.includes(
        `${formatBytes(memoryReserved, 'MiB')} / ${formatBytes(
          memoryTotal,
          'MiB'
        )}`
      )
    );
    assert.ok(
      TopoVizDatacenter.label.includes(
        `${formatHertz(cpuReserved, 'MHz')} / ${formatHertz(cpuTotal, 'MHz')}`
      )
    );
  });

  test('when @isSingleColumn is true, the FlexMasonry layout gets one column, otherwise it gets two', async function (assert) {
    this.setProperties(
      commonProps({
        isSingleColumn: true,
        datacenter: {
          name: 'dc1',
          nodes: [
            nodeGen('node-1', 'dc1', 1000, 500),
            nodeGen('node-2', 'dc1', 1000, 500),
          ],
        },
      })
    );

    await render(commonTemplate);

    assert.ok(find('[data-test-flex-masonry].flex-masonry-columns-1'));

    this.set('isSingleColumn', false);
    assert.ok(find('[data-test-flex-masonry].flex-masonry-columns-2'));
  });

  test('args get passed down to the TopViz::Node children', async function (assert) {
    assert.expect(4);

    const heightSpy = sinon.spy();
    this.setProperties(
      commonProps({
        isDense: true,
        heightScale: (...args) => {
          heightSpy(...args);
          return 50;
        },
        datacenter: {
          name: 'dc1',
          nodes: [
            nodeGen('node-1', 'dc1', 1000, 500, [{ memory: 100, cpu: 300 }]),
          ],
        },
      })
    );

    await render(commonTemplate);

    TopoVizDatacenter.nodes[0].as(async (TopoVizNode) => {
      assert.notOk(TopoVizNode.labelIsPresent);
      assert.ok(heightSpy.calledWith(this.datacenter.nodes[0].memory));

      await TopoVizNode.selectNode();
      assert.ok(this.onNodeSelect.calledWith(this.datacenter.nodes[0]));

      await TopoVizNode.memoryRects[0].select();
      assert.ok(
        this.onAllocationSelect.calledWith(
          this.datacenter.nodes[0].allocations[0]
        )
      );
    });
  });
});
