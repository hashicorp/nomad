import EmberObject from '@ember/object';
import { assign } from '@ember/polyfills';
import wait from 'ember-test-helpers/wait';
import { module, test } from 'ember-qunit';
import sinon from 'sinon';
import Pretender from 'pretender';
import AllocationStatsTracker, { stats } from 'nomad-ui/utils/classes/allocation-stats-tracker';
import fetch from 'nomad-ui/utils/fetch';
import statsTrackerFrameMissingBehavior from './behaviors/stats-tracker-frame-missing';

module('Unit | Util | AllocationStatsTracker');

const refDate = Date.now() * 1000000;
const makeDate = ts => new Date(ts / 1000000);

const MockAllocation = overrides =>
  assign(
    {
      id: 'some-identifier',
      taskGroup: {
        reservedCPU: 200,
        reservedMemory: 512,
        tasks: [
          {
            name: 'service',
            reservedCPU: 100,
            reservedMemory: 256,
          },
          {
            name: 'log-shipper',
            reservedCPU: 50,
            reservedMemory: 128,
          },
          {
            name: 'sidecar',
            reservedCPU: 50,
            reservedMemory: 128,
          },
        ],
      },
    },
    overrides
  );

const mockFrame = step => ({
  ResourceUsage: {
    CpuStats: {
      TotalTicks: step + 100,
    },
    MemoryStats: {
      RSS: (step + 400) * 1024 * 1024,
    },
  },
  Tasks: {
    service: {
      ResourceUsage: {
        CpuStats: {
          TotalTicks: step + 50,
        },
        MemoryStats: {
          RSS: (step + 100) * 1024 * 1024,
        },
      },
      Timestamp: refDate + step,
    },
    'log-shipper': {
      ResourceUsage: {
        CpuStats: {
          TotalTicks: step + 25,
        },
        MemoryStats: {
          RSS: (step + 50) * 1024 * 1024,
        },
      },
      Timestamp: refDate + step * 10,
    },
    sidecar: {
      ResourceUsage: {
        CpuStats: {
          TotalTicks: step + 26,
        },
        MemoryStats: {
          RSS: (step + 51) * 1024 * 1024,
        },
      },
      Timestamp: refDate + step * 100,
    },
  },
  Timestamp: refDate + step * 1000,
});

test('the AllocationStatsTracker constructor expects a fetch definition and an allocation', function(assert) {
  const tracker = AllocationStatsTracker.create();
  assert.throws(
    () => {
      tracker.fetch();
    },
    /StatsTrackers need a fetch method/,
    'Polling does not work without a fetch method provided'
  );
});

test('the url property is computed based off the allocation id', function(assert) {
  const allocation = MockAllocation();
  const tracker = AllocationStatsTracker.create({ fetch, allocation });

  assert.equal(
    tracker.get('url'),
    `/v1/client/allocation/${allocation.id}/stats`,
    'Url is derived from the allocation id'
  );
});

test('reservedCPU and reservedMemory properties come from the allocation', function(assert) {
  const allocation = MockAllocation();
  const tracker = AllocationStatsTracker.create({ fetch, allocation });

  assert.equal(
    tracker.get('reservedCPU'),
    allocation.taskGroup.reservedCPU,
    'reservedCPU comes from the allocation task group'
  );
  assert.equal(
    tracker.get('reservedMemory'),
    allocation.taskGroup.reservedMemory,
    'reservedMemory comes from the allocation task group'
  );
});

test('the tasks list comes from the allocation', function(assert) {
  const allocation = MockAllocation();
  const tracker = AllocationStatsTracker.create({ fetch, allocation });

  assert.equal(
    tracker.get('tasks.length'),
    allocation.taskGroup.tasks.length,
    'tasks matches lengths with the allocation task group'
  );
  allocation.taskGroup.tasks.forEach(task => {
    const trackerTask = tracker.get('tasks').findBy('task', task.name);
    assert.equal(trackerTask.reservedCPU, task.reservedCPU, `CPU matches for task ${task.name}`);
    assert.equal(
      trackerTask.reservedMemory,
      task.reservedMemory,
      `Memory matches for task ${task.name}`
    );
  });
});

test('poll results in requesting the url and calling append with the resulting JSON', function(assert) {
  const allocation = MockAllocation();
  const tracker = AllocationStatsTracker.create({ fetch, allocation, append: sinon.spy() });
  const mockFrame = {
    Some: {
      data: ['goes', 'here'],
      twelve: 12,
    },
  };

  const server = new Pretender(function() {
    this.get('/v1/client/allocation/:id/stats', () => [200, {}, JSON.stringify(mockFrame)]);
  });

  tracker.get('poll').perform();

  assert.equal(server.handledRequests.length, 1, 'Only one request was made');
  assert.equal(
    server.handledRequests[0].url,
    `/v1/client/allocation/${allocation.id}/stats`,
    'The correct URL was requested'
  );

  return wait().then(() => {
    assert.ok(
      tracker.append.calledWith(mockFrame),
      'The JSON response was passed onto append as a POJO'
    );

    server.shutdown();
  });
});

test('append appropriately maps a data frame to the tracked stats for cpu and memory for the allocation as well as individual tasks', function(assert) {
  const allocation = MockAllocation();
  const tracker = AllocationStatsTracker.create({ fetch, allocation });

  assert.deepEqual(tracker.get('cpu'), [], 'No tracked cpu yet');
  assert.deepEqual(tracker.get('memory'), [], 'No tracked memory yet');

  assert.deepEqual(
    tracker.get('tasks'),
    [
      { task: 'service', reservedCPU: 100, reservedMemory: 256, cpu: [], memory: [] },
      { task: 'log-shipper', reservedCPU: 50, reservedMemory: 128, cpu: [], memory: [] },
      { task: 'sidecar', reservedCPU: 50, reservedMemory: 128, cpu: [], memory: [] },
    ],
    'tasks represents the tasks for the allocation with no stats yet'
  );

  tracker.append(mockFrame(1));

  assert.deepEqual(
    tracker.get('cpu'),
    [{ timestamp: makeDate(refDate + 1000), used: 101, percent: 101 / 200 }],
    'One frame of cpu'
  );
  assert.deepEqual(
    tracker.get('memory'),
    [
      {
        timestamp: makeDate(refDate + 1000),
        used: 401 * 1024 * 1024,
        percent: 401 / 512,
      },
    ],
    'One frame of memory'
  );

  assert.deepEqual(
    tracker.get('tasks'),
    [
      {
        task: 'service',
        reservedCPU: 100,
        reservedMemory: 256,
        cpu: [{ timestamp: makeDate(refDate + 1), used: 51, percent: 51 / 100 }],
        memory: [
          {
            timestamp: makeDate(refDate + 1),
            used: 101 * 1024 * 1024,
            percent: 101 / 256,
          },
        ],
      },
      {
        task: 'log-shipper',
        reservedCPU: 50,
        reservedMemory: 128,
        cpu: [{ timestamp: makeDate(refDate + 10), used: 26, percent: 26 / 50 }],
        memory: [
          {
            timestamp: makeDate(refDate + 10),
            used: 51 * 1024 * 1024,
            percent: 51 / 128,
          },
        ],
      },
      {
        task: 'sidecar',
        reservedCPU: 50,
        reservedMemory: 128,
        cpu: [{ timestamp: makeDate(refDate + 100), used: 27, percent: 27 / 50 }],
        memory: [
          {
            timestamp: makeDate(refDate + 100),
            used: 52 * 1024 * 1024,
            percent: 52 / 128,
          },
        ],
      },
    ],
    'tasks represents the tasks for the allocation, each with one frame of stats'
  );

  tracker.append(mockFrame(2));

  assert.deepEqual(
    tracker.get('cpu'),
    [
      { timestamp: makeDate(refDate + 1000), used: 101, percent: 101 / 200 },
      { timestamp: makeDate(refDate + 2000), used: 102, percent: 102 / 200 },
    ],
    'Two frames of cpu'
  );
  assert.deepEqual(
    tracker.get('memory'),
    [
      { timestamp: makeDate(refDate + 1000), used: 401 * 1024 * 1024, percent: 401 / 512 },
      { timestamp: makeDate(refDate + 2000), used: 402 * 1024 * 1024, percent: 402 / 512 },
    ],
    'Two frames of memory'
  );

  assert.deepEqual(
    tracker.get('tasks'),
    [
      {
        task: 'service',
        reservedCPU: 100,
        reservedMemory: 256,
        cpu: [
          { timestamp: makeDate(refDate + 1), used: 51, percent: 51 / 100 },
          { timestamp: makeDate(refDate + 2), used: 52, percent: 52 / 100 },
        ],
        memory: [
          { timestamp: makeDate(refDate + 1), used: 101 * 1024 * 1024, percent: 101 / 256 },
          { timestamp: makeDate(refDate + 2), used: 102 * 1024 * 1024, percent: 102 / 256 },
        ],
      },
      {
        task: 'log-shipper',
        reservedCPU: 50,
        reservedMemory: 128,
        cpu: [
          { timestamp: makeDate(refDate + 10), used: 26, percent: 26 / 50 },
          { timestamp: makeDate(refDate + 20), used: 27, percent: 27 / 50 },
        ],
        memory: [
          { timestamp: makeDate(refDate + 10), used: 51 * 1024 * 1024, percent: 51 / 128 },
          { timestamp: makeDate(refDate + 20), used: 52 * 1024 * 1024, percent: 52 / 128 },
        ],
      },
      {
        task: 'sidecar',
        reservedCPU: 50,
        reservedMemory: 128,
        cpu: [
          { timestamp: makeDate(refDate + 100), used: 27, percent: 27 / 50 },
          { timestamp: makeDate(refDate + 200), used: 28, percent: 28 / 50 },
        ],
        memory: [
          { timestamp: makeDate(refDate + 100), used: 52 * 1024 * 1024, percent: 52 / 128 },
          { timestamp: makeDate(refDate + 200), used: 53 * 1024 * 1024, percent: 53 / 128 },
        ],
      },
    ],
    'tasks represents the tasks for the allocation, each with two frames of stats'
  );
});

test('each stat list has maxLength equal to bufferSize', function(assert) {
  const allocation = MockAllocation();
  const bufferSize = 10;
  const tracker = AllocationStatsTracker.create({ fetch, allocation, bufferSize });

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
    +makeDate(refDate + 11000),
    'Old frames are removed in favor of newer ones'
  );
  assert.equal(
    +tracker.get('memory')[0].timestamp,
    +makeDate(refDate + 11000),
    'Old frames are removed in favor of newer ones'
  );

  tracker.get('tasks').forEach(task => {
    assert.equal(
      task.cpu.length,
      bufferSize,
      `20 calls to append, only ${bufferSize} frames in the stats array`
    );
    assert.equal(
      task.memory.length,
      bufferSize,
      `20 calls to append, only ${bufferSize} frames in the stats array`
    );
  });

  assert.equal(
    +tracker.get('tasks').findBy('task', 'service').cpu[0].timestamp,
    +makeDate(refDate + 11),
    'Old frames are removed in favor of newer ones'
  );
  assert.equal(
    +tracker.get('tasks').findBy('task', 'service').memory[0].timestamp,
    +makeDate(refDate + 11),
    'Old frames are removed in favor of newer ones'
  );

  assert.equal(
    +tracker.get('tasks').findBy('task', 'log-shipper').cpu[0].timestamp,
    +makeDate(refDate + 110),
    'Old frames are removed in favor of newer ones'
  );
  assert.equal(
    +tracker.get('tasks').findBy('task', 'log-shipper').memory[0].timestamp,
    +makeDate(refDate + 110),
    'Old frames are removed in favor of newer ones'
  );

  assert.equal(
    +tracker.get('tasks').findBy('task', 'sidecar').cpu[0].timestamp,
    +makeDate(refDate + 1100),
    'Old frames are removed in favor of newer ones'
  );
  assert.equal(
    +tracker.get('tasks').findBy('task', 'sidecar').memory[0].timestamp,
    +makeDate(refDate + 1100),
    'Old frames are removed in favor of newer ones'
  );
});

test('the stats computed property macro constructs an AllocationStatsTracker based on an allocationProp and a fetch definition', function(assert) {
  const allocation = MockAllocation();
  const fetchSpy = sinon.spy();

  const SomeClass = EmberObject.extend({
    stats: stats('alloc', function() {
      return () => fetchSpy(this);
    }),
  });
  const someObject = SomeClass.create({
    alloc: allocation,
  });

  assert.equal(
    someObject.get('stats.url'),
    `/v1/client/allocation/${allocation.id}/stats`,
    'stats computed property macro creates an AllocationStatsTracker'
  );

  someObject.get('stats').fetch();

  assert.ok(
    fetchSpy.calledWith(someObject),
    'the fetch factory passed into the macro gets called to assign a bound version of fetch to the AllocationStatsTracker instance'
  );
});

test('changing the value of the allocationProp constructs a new AllocationStatsTracker', function(assert) {
  const alloc1 = MockAllocation();
  const alloc2 = MockAllocation();
  const SomeClass = EmberObject.extend({
    stats: stats('alloc', () => fetch),
  });

  const someObject = SomeClass.create({
    alloc: alloc1,
  });

  const stats1 = someObject.get('stats');

  someObject.set('alloc', alloc2);
  const stats2 = someObject.get('stats');

  assert.notOk(
    stats1 === stats2,
    'Changing the value of alloc results in creating a new AllocationStatsTracker instance'
  );
});

statsTrackerFrameMissingBehavior({
  resourceName: 'allocation',
  ResourceConstructor: MockAllocation,
  TrackerConstructor: AllocationStatsTracker,
  mockFrame,
  compileResources(frame) {
    const timestamp = makeDate(frame.Timestamp);
    const cpu = frame.ResourceUsage.CpuStats.TotalTicks;
    const memory = frame.ResourceUsage.MemoryStats.RSS;
    return [
      {
        timestamp,
        used: cpu,
        percent: cpu / 200,
      },
      {
        timestamp,
        used: memory,
        percent: memory / 1024 / 1024 / 512,
      },
    ];
  },
});
