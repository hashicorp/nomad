/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import EmberObject, { get, computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import RollingArray from 'nomad-ui/utils/classes/rolling-array';
import AbstractStatsTracker from 'nomad-ui/utils/classes/abstract-stats-tracker';
import classic from 'ember-classic-decorator';

const percent = (numerator, denominator) => {
  if (!numerator || !denominator) {
    return 0;
  }
  return numerator / denominator;
};

const empty = (ts) => ({ timestamp: ts, used: null, percent: null });

// Tasks are sorted by their lifecycle phase in this order:
const sortMap = [
  'main',
  'prestart-sidecar',
  'poststart-sidecar',
  'prestart-ephemeral',
  'poststart-ephemeral',
  'poststop',
].reduce((map, phase, index) => {
  map[phase] = index;
  return map;
}, {});

const taskPrioritySort = (a, b) =>
  sortMap[a.lifecycleName] - sortMap[b.lifecycleName];

// Select the value for memory usage.
// Must match logic in command/alloc_status.go.
const memoryUsed = (frame) =>
  frame.ResourceUsage.MemoryStats.RSS ||
  frame.ResourceUsage.MemoryStats.Usage ||
  0;

@classic
class AllocationStatsTracker extends EmberObject.extend(AbstractStatsTracker) {
  // Set via the stats computed property macro
  allocation = null;

  @computed('allocation.id')
  get url() {
    return `/v1/client/allocation/${this.get('allocation.id')}/stats`;
  }

  append(frame) {
    const timestamp = new Date(Math.floor(frame.Timestamp / 1000000));

    const cpuUsed = Math.floor(frame.ResourceUsage.CpuStats.TotalTicks) || 0;
    this.cpu.pushObject({
      timestamp,
      used: cpuUsed,
      percent: percent(cpuUsed, this.reservedCPU),
    });

    const memUsed = memoryUsed(frame);
    this.memory.pushObject({
      timestamp,
      used: memUsed,
      percent: percent(memUsed / 1024 / 1024, this.reservedMemory),
    });

    let aggregateCpu = 0;
    let aggregateMemory = 0;
    for (var stats of this.tasks) {
      const taskFrame = frame.Tasks[stats.task];

      // If the task is not present in the frame data (because it hasn't started or
      // it has already stopped), just keep going.
      if (!taskFrame) continue;

      const frameTimestamp = new Date(
        Math.floor(taskFrame.Timestamp / 1000000)
      );

      const taskCpuUsed =
        Math.floor(taskFrame.ResourceUsage.CpuStats.TotalTicks) || 0;
      const percentCpuTotal = percent(taskCpuUsed, this.reservedCPU);
      stats.cpu.pushObject({
        timestamp: frameTimestamp,
        used: taskCpuUsed,
        percent: percent(taskCpuUsed, stats.reservedCPU),
        percentTotal: percentCpuTotal,
        percentStack: percentCpuTotal + aggregateCpu,
      });

      const taskMemoryUsed = memoryUsed(taskFrame);
      const percentMemoryTotal = percent(
        taskMemoryUsed / 1024 / 1024,
        this.reservedMemory
      );
      stats.memory.pushObject({
        timestamp: frameTimestamp,
        used: taskMemoryUsed,
        percent: percent(taskMemoryUsed / 1024 / 1024, stats.reservedMemory),
        percentTotal: percentMemoryTotal,
        percentStack: percentMemoryTotal + aggregateMemory,
      });

      aggregateCpu += percentCpuTotal;
      aggregateMemory += percentMemoryTotal;
    }
  }

  pause() {
    const ts = new Date();
    this.memory.pushObject(empty(ts));
    this.cpu.pushObject(empty(ts));
    this.tasks.forEach((task) => {
      task.memory.pushObject(empty(ts));
      task.cpu.pushObject(empty(ts));
    });
  }

  // Static figures, denominators for stats
  @alias('allocation.taskGroup.reservedCPU') reservedCPU;
  @alias('allocation.taskGroup.reservedMemory') reservedMemory;

  // Dynamic figures, collected over time
  // []{ timestamp: Date, used: Number, percent: Number }
  @computed('allocation', 'bufferSize')
  get cpu() {
    return RollingArray(this.bufferSize);
  }

  @computed('allocation', 'bufferSize')
  get memory() {
    return RollingArray(this.bufferSize);
  }

  @computed('allocation.taskGroup.tasks', 'bufferSize')
  get tasks() {
    const bufferSize = this.bufferSize;
    const tasks = this.get('allocation.taskGroup.tasks') || [];
    return tasks
      .slice()
      .sort(taskPrioritySort)
      .map((task) => ({
        task: get(task, 'name'),

        // Static figures, denominators for stats
        reservedCPU: get(task, 'reservedCPU'),
        reservedMemory: get(task, 'reservedMemory'),

        // Dynamic figures, collected over time
        // []{ timestamp: Date, used: Number, percent: Number }
        cpu: RollingArray(bufferSize),
        memory: RollingArray(bufferSize),
      }));
  }
}

export default AllocationStatsTracker;

export function stats(allocationProp, fetch) {
  return computed(allocationProp, function () {
    return AllocationStatsTracker.create({
      fetch: fetch.call(this),
      allocation: this.get(allocationProp),
    });
  });
}
