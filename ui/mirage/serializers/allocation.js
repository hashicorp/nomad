import ApplicationSerializer from './application';
import { arrToObj } from '../utils';

export default ApplicationSerializer.extend({
  embed: true,
  include: ['taskStates', 'taskResources'],

  serialize() {
    var json = ApplicationSerializer.prototype.serialize.apply(this, arguments);
    if (json instanceof Array) {
      json.forEach(serializeAllocation);
    } else {
      serializeAllocation(json);
    }
    return json;
  },
});

function serializeAllocation(allocation) {
  allocation.TaskStates = allocation.TaskStates.reduce(arrToObj('Name'), {});
  allocation.Resources = allocation.TaskResources.mapBy('Resources').reduce(
    (hash, resources) => {
      ['CPU', 'DiskMB', 'IOPS', 'MemoryMB'].forEach(key => (hash[key] += resources[key]));
      hash.Networks = resources.Networks;
      hash.Ports = resources.Ports;
      return hash;
    },
    { CPU: 0, DiskMB: 0, IOPS: 0, MemoryMB: 0 }
  );
  allocation.TaskResources = allocation.TaskResources.reduce(arrToObj('Name', 'Resources'), {});
}
