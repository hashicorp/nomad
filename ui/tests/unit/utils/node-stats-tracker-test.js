/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import EmberObject from '@ember/object';
import { assign } from '@ember/polyfills';
import { module, test } from 'qunit';
import sinon from 'sinon';
import Pretender from 'pretender';
import NodeStatsTracker, {
  stats,
} from 'nomad-ui/utils/classes/node-stats-tracker';
import fetch from 'nomad-ui/utils/fetch';
import statsTrackerFrameMissingBehavior from './behaviors/stats-tracker-frame-missing';

import { settled } from '@ember/test-helpers';

module('Unit | Util | NodeStatsTracker', function () {
  const refDate = Date.now() * 1000000;
  const makeDate = (ts) => new Date(ts / 1000000);

  const MockNode = (overrides) =>
    assign(
      {
        id: 'some-identifier',
        resources: {
          cpu: 2000,
          memory: 4096,
        },
      },
      overrides
    );

  const mockFrame = (step) => ({
    CPUTicksConsumed: step + 1000,
    Memory: {
      Used: (step + 2048) * 1024 * 1024,
    },
    Timestamp: refDate + step,
  });

  test('the NodeStatsTracker constructor expects a fetch definition and a node', async function (assert) {
    const tracker = NodeStatsTracker.create();
    assert.throws(
      () => {
        tracker.fetch();
      },
      /StatsTrackers need a fetch method/,
      'Polling does not work without a fetch method provided'
    );
  });

  test('the url property is computed based off the node id', async function (assert) {
    const node = MockNode();
    const tracker = NodeStatsTracker.create({ fetch, node });

    assert.equal(
      tracker.get('url'),
      `/v1/client/stats?node_id=${node.id}`,
      'Url is derived from the node id'
    );
  });

  test('reservedCPU and reservedMemory properties come from the node', async function (assert) {
    const node = MockNode();
    const tracker = NodeStatsTracker.create({ fetch, node });

    assert.equal(
      tracker.get('reservedCPU'),
      node.resources.cpu,
      'reservedCPU comes from the node'
    );
    assert.equal(
      tracker.get('reservedMemory'),
      node.resources.memory,
      'reservedMemory comes from the node'
    );
  });

  test('poll results in requesting the url and calling append with the resulting JSON', async function (assert) {
    const node = MockNode();
    const tracker = NodeStatsTracker.create({
      fetch,
      node,
      append: sinon.spy(),
    });
    const mockFrame = {
      Some: {
        data: ['goes', 'here'],
        twelve: 12,
      },
    };

    const server = new Pretender(function () {
      this.get('/v1/client/stats', () => [200, {}, JSON.stringify(mockFrame)]);
    });

    tracker.get('poll').perform();

    assert.equal(server.handledRequests.length, 1, 'Only one request was made');
    assert.equal(
      server.handledRequests[0].url,
      `/v1/client/stats?node_id=${node.id}`,
      'The correct URL was requested'
    );

    await settled();
    assert.ok(
      tracker.append.calledWith(mockFrame),
      'The JSON response was passed into append as a POJO'
    );

    server.shutdown();
  });

  test('append appropriately maps a data frame to the tracked stats for cpu and memory for the node', async function (assert) {
    const node = MockNode();
    const tracker = NodeStatsTracker.create({ fetch, node });

    assert.deepEqual(tracker.get('cpu'), [], 'No tracked cpu yet');
    assert.deepEqual(tracker.get('memory'), [], 'No tracked memory yet');

    tracker.append(mockFrame(1));

    assert.deepEqual(
      tracker.get('cpu'),
      [{ timestamp: makeDate(refDate + 1), used: 1001, percent: 1001 / 2000 }],
      'One frame of cpu'
    );

    assert.deepEqual(
      tracker.get('memory'),
      [
        {
          timestamp: makeDate(refDate + 1),
          used: 2049 * 1024 * 1024,
          percent: 2049 / 4096,
        },
      ],
      'One frame of memory'
    );

    tracker.append(mockFrame(2));

    assert.deepEqual(
      tracker.get('cpu'),
      [
        { timestamp: makeDate(refDate + 1), used: 1001, percent: 1001 / 2000 },
        { timestamp: makeDate(refDate + 2), used: 1002, percent: 1002 / 2000 },
      ],
      'Two frames of cpu'
    );

    assert.deepEqual(
      tracker.get('memory'),
      [
        {
          timestamp: makeDate(refDate + 1),
          used: 2049 * 1024 * 1024,
          percent: 2049 / 4096,
        },
        {
          timestamp: makeDate(refDate + 2),
          used: 2050 * 1024 * 1024,
          percent: 2050 / 4096,
        },
      ],
      'Two frames of memory'
    );
  });

  test('each stat list has maxLength equal to bufferSize', async function (assert) {
    const node = MockNode();
    const bufferSize = 10;
    const tracker = NodeStatsTracker.create({ fetch, node, bufferSize });

    for (let i = 1; i <= 20; i++) {
      tracker.append(mockFrame(i));
    }

    assert.equal(
      tracker.get('cpu.length'),
      bufferSize,
      `20 calls to append, only ${bufferSize} frames in the stats array`
    );
    assert.equal(
      tracker.get('memory.length'),
      bufferSize,
      `20 calls to append, only ${bufferSize} frames in the stats array`
    );

    assert.equal(
      +tracker.get('cpu')[0].timestamp,
      +makeDate(refDate + 11),
      'Old frames are removed in favor of newer ones'
    );
    assert.equal(
      +tracker.get('memory')[0].timestamp,
      +makeDate(refDate + 11),
      'Old frames are removed in favor of newer ones'
    );
  });

  test('the stats computed property macro constructs a NodeStatsTracker based on a nodeProp and a fetch definition', async function (assert) {
    const node = MockNode();
    const fetchSpy = sinon.spy();

    const SomeClass = EmberObject.extend({
      stats: stats('theNode', function () {
        return () => fetchSpy(this);
      }),
    });
    const someObject = SomeClass.create({
      theNode: node,
    });

    assert.equal(
      someObject.get('stats.url'),
      `/v1/client/stats?node_id=${node.id}`,
      'stats computed property macro creates a NodeStatsTracker'
    );

    someObject.get('stats').fetch();

    assert.ok(
      fetchSpy.calledWith(someObject),
      'the fetch factory passed into the macro gets called to assign a bound version of fetch to the NodeStatsTracker instance'
    );
  });

  test('changing the value of the nodeProp constructs a new NodeStatsTracker', async function (assert) {
    const node1 = MockNode();
    const node2 = MockNode();
    const SomeClass = EmberObject.extend({
      stats: stats('theNode', () => fetch),
    });

    const someObject = SomeClass.create({
      theNode: node1,
    });

    const stats1 = someObject.get('stats');

    someObject.set('theNode', node2);
    const stats2 = someObject.get('stats');

    assert.notStrictEqual(
      stats1,
      stats2,
      'Changing the value of the node results in creating a new NodeStatsTracker instance'
    );
  });

  statsTrackerFrameMissingBehavior({
    resourceName: 'node',
    ResourceConstructor: MockNode,
    TrackerConstructor: NodeStatsTracker,
    mockFrame,
    compileResources(frame) {
      const timestamp = makeDate(frame.Timestamp);
      return [
        {
          timestamp,
          used: frame.CPUTicksConsumed,
          percent: frame.CPUTicksConsumed / 2000,
        },
        {
          timestamp,
          used: frame.Memory.Used,
          percent: frame.Memory.Used / 1024 / 1024 / 4096,
        },
      ];
    },
  });
});
