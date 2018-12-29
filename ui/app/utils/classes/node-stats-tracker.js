import EmberObject, { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import RollingArray from 'nomad-ui/utils/classes/rolling-array';
import AbstractStatsTracker from 'nomad-ui/utils/classes/abstract-stats-tracker';

const percent = (numerator, denominator) => {
  if (!numerator || !denominator) {
    return 0;
  }
  return numerator / denominator;
};

const empty = ts => ({ timestamp: ts, used: null, percent: null });

const NodeStatsTracker = EmberObject.extend(AbstractStatsTracker, {
  // Set via the stats computed property macro
  node: null,

  url: computed('node', function() {
    return `/v1/client/stats?node_id=${this.get('node.id')}`;
  }),

  append(frame) {
    const timestamp = new Date(Math.floor(frame.Timestamp / 1000000));

    const cpuUsed = Math.floor(frame.CPUTicksConsumed) || 0;
    this.get('cpu').pushObject({
      timestamp,
      used: cpuUsed,
      percent: percent(cpuUsed, this.get('reservedCPU')),
    });

    const memoryUsed = frame.Memory.Used;
    this.get('memory').pushObject({
      timestamp,
      used: memoryUsed,
      percent: percent(memoryUsed / 1024 / 1024, this.get('reservedMemory')),
    });
  },

  pause() {
    const ts = new Date();
    this.get('memory').pushObject(empty(ts));
    this.get('cpu').pushObject(empty(ts));
  },

  // Static figures, denominators for stats
  reservedCPU: alias('node.resources.cpu'),
  reservedMemory: alias('node.resources.memory'),

  // Dynamic figures, collected over time
  // []{ timestamp: Date, used: Number, percent: Number }
  cpu: computed('node', function() {
    return RollingArray(this.get('bufferSize'));
  }),
  memory: computed('node', function() {
    return RollingArray(this.get('bufferSize'));
  }),
});

export default NodeStatsTracker;

export function stats(nodeProp, fetch) {
  return computed(nodeProp, function() {
    return NodeStatsTracker.create({
      fetch: fetch.call(this),
      node: this.get(nodeProp),
    });
  });
}
