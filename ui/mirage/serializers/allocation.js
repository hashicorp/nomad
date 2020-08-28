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
  allocation.TaskResources = allocation.TaskResources.reduce(arrToObj('Name', 'Resources'), {});
}
