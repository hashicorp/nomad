import EmberObject, { computed } from '@ember/object';
import { alias, readOnly } from '@ember/object/computed';

export default EmberObject.extend({
  allocation: null,
  stats: null,

  reservedMemory: alias('allocation.taskGroup.reservedMemory'),
  reservedCPU: alias('allocation.taskGroup.reservedCPU'),

  memoryUsed: readOnly('stats.ResourceUsage.MemoryStats.RSS'),
  cpuUsed: computed('stats.ResourceUsage.CpuStats.TotalTicks', function() {
    return Math.floor(this.get('stats.ResourceUsage.CpuStats.TotalTicks') || 0);
  }),

  percentMemory: computed('reservedMemory', 'memoryUsed', function() {
    const used = this.get('memoryUsed') / 1024 / 1024;
    const total = this.get('reservedMemory');
    if (!total || !used) {
      return 0;
    }
    return used / total;
  }),

  percentCPU: computed('reservedCPU', 'cpuUsed', function() {
    const used = this.get('cpuUsed');
    const total = this.get('reservedCPU');
    if (!total || !used) {
      return 0;
    }
    return used / total;
  }),
});
