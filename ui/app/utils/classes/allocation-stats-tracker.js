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

const empty = ts => ({ timestamp: ts, used: null, percent: null });

@classic
class AllocationStatsTracker extends EmberObject.extend(AbstractStatsTracker) {
  // Set via the stats computed property macro
  allocation = null;

  @computed('allocation')
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

    const memoryUsed = frame.ResourceUsage.MemoryStats.RSS;
    this.memory.pushObject({
      timestamp,
      used: memoryUsed,
      percent: percent(memoryUsed / 1024 / 1024, this.reservedMemory),
    });

    for (var taskName in frame.Tasks) {
      const taskFrame = frame.Tasks[taskName];
      const stats = this.tasks.findBy('task', taskName);

      // If for whatever reason there is a task in the frame data that isn't in the
      // allocation, don't attempt to append data for the task.
      if (!stats) continue;

      const frameTimestamp = new Date(Math.floor(taskFrame.Timestamp / 1000000));

      const taskCpuUsed = Math.floor(taskFrame.ResourceUsage.CpuStats.TotalTicks) || 0;
      stats.cpu.pushObject({
        timestamp: frameTimestamp,
        used: taskCpuUsed,
        percent: percent(taskCpuUsed, stats.reservedCPU),
      });

      const taskMemoryUsed = taskFrame.ResourceUsage.MemoryStats.RSS;
      stats.memory.pushObject({
        timestamp: frameTimestamp,
        used: taskMemoryUsed,
        percent: percent(taskMemoryUsed / 1024 / 1024, stats.reservedMemory),
      });
    }
  }

  pause() {
    const ts = new Date();
    this.memory.pushObject(empty(ts));
    this.cpu.pushObject(empty(ts));
    this.tasks.forEach(task => {
      task.memory.pushObject(empty(ts));
      task.cpu.pushObject(empty(ts));
    });
  }

  // Static figures, denominators for stats
  @alias('allocation.taskGroup.reservedCPU') reservedCPU;
  @alias('allocation.taskGroup.reservedMemory') reservedMemory;

  // Dynamic figures, collected over time
  // []{ timestamp: Date, used: Number, percent: Number }
  @computed('allocation')
  get cpu() {
    return RollingArray(this.bufferSize);
  }

  @computed('allocation')
  get memory() {
    return RollingArray(this.bufferSize);
  }

  @computed('allocation')
  get tasks() {
    const bufferSize = this.bufferSize;
    const tasks = this.get('allocation.taskGroup.tasks') || [];
    return tasks.map(task => ({
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
  return computed(allocationProp, function() {
    return AllocationStatsTracker.create({
      fetch: fetch.call(this),
      allocation: this.get(allocationProp),
    });
  });
}
