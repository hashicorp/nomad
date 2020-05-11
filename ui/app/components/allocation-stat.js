import Component from '@ember/component';
import { computed } from '@ember/object';
import { alias } from '@ember/object/computed';
import { formatBytes } from 'nomad-ui/helpers/format-bytes';

export default Component.extend({
  tagName: '',

  allocation: null,
  statsTracker: null,
  isLoading: false,
  error: null,
  metric: 'memory', // Either memory or cpu

  statClass: computed('metric', function() {
    return this.metric === 'cpu' ? 'is-info' : 'is-danger';
  }),

  cpu: alias('statsTracker.cpu.lastObject'),
  memory: alias('statsTracker.memory.lastObject'),

  stat: computed('metric', 'cpu', 'memory', function() {
    const { metric } = this;
    if (metric === 'cpu' || metric === 'memory') {
      return this[this.metric];
    }
  }),

  formattedStat: computed('metric', 'stat.used', function() {
    if (!this.stat) return;
    if (this.metric === 'memory') return formatBytes([this.stat.used]);
    return this.stat.used;
  }),

  formattedReserved: computed(
    'metric',
    'statsTracker.reservedMemory',
    'statsTracker.reservedCPU',
    function() {
      if (this.metric === 'memory') return `${this.statsTracker.reservedMemory} MiB`;
      if (this.metric === 'cpu') return `${this.statsTracker.reservedCPU} MHz`;
    }
  ),
});
