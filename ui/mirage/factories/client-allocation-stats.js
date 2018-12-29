import { Factory } from 'ember-cli-mirage';

export default Factory.extend({
  resourceUsage: generateResources,

  _taskNames: () => [], // Set by allocation

  tasks() {
    var hash = {};
    this._taskNames.forEach(task => {
      hash[task] = {
        Pids: null,
        ResourceUsage: generateResources(),
        Timestamp: Date.now(),
      };
    });
    return hash;
  },
});

function generateResources() {
  return {
    CpuStats: {
      Measured: ['Throttled Periods', 'Throttled Time', 'Percent'],
      Percent: 0.14159538847117795,
      SystemMode: 0,
      ThrottledPeriods: 0,
      ThrottledTime: 0,
      TotalTicks: 3.256693934837093,
      UserMode: 0,
    },
    MemoryStats: {
      Cache: 1744896,
      KernelMaxUsage: 0,
      KernelUsage: 0,
      MaxUsage: 4710400,
      Measured: ['RSS', 'Cache', 'Swap', 'Max Usage'],
      RSS: 1486848,
      Swap: 0,
    },
  };
}
