import { Factory } from 'ember-cli-mirage';
import generateResources from '../data/generate-resources';

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
