import { Factory, trait } from 'ember-cli-mirage';
import generateResources from '../data/generate-resources';

export default Factory.extend({
  _taskNames: () => [], // Set by allocation
  _tasks: () => [], // Optionally set by the allocation

  timestamp: () => Date.now() * 1000000,

  highUsage: trait({
    tasks() {
      var hash = {};
      this._taskNames.forEach(task => {
        const resources = this._tasks.find(t => t.name === task).Resources;
        hash[task] = {
          Pids: null,
          ResourceUsage: generateResources(
            resources.CPU * 0.9,
            resources.MemoryMB * 1024 * 1024 * 0.9
          ),
          Timestamp: Date.now() * 1000000,
        };
      });
      return hash;
    },
  }),

  tasks() {
    var hash = {};
    this._taskNames.forEach(task => {
      hash[task] = {
        Pids: null,
        ResourceUsage: generateResources(),
        Timestamp: Date.now() * 1000000,
      };
    });
    return hash;
  },

  resourceUsage() {
    const resources = generateResources();

    // Zero out the pertinent metrics to then aggregate
    resources.CpuStats.TotalTicks = 0;
    resources.MemoryStats.RSS = 0;

    return Object.keys(this.tasks).reduce((hash, taskName) => {
      const task = this.tasks[taskName];
      hash.CpuStats.TotalTicks += task.ResourceUsage.CpuStats.TotalTicks;
      hash.MemoryStats.RSS += task.ResourceUsage.MemoryStats.RSS;
      return hash;
    }, resources);
  },
});
