import ApplicationSerializer from './application';
import { arrToObj } from '../utils';

export default ApplicationSerializer.extend({
  //   embed: true,
  //   include: ['deploymentTaskGroupSummaries'],

  serialize() {
    var json = ApplicationSerializer.prototype.serialize.apply(this, arguments);
    if (json instanceof Array) {
      json.forEach(serializeService);
    } else {
      serializeService(json);
    }
    return json;
  },
});

function serializeService(service) {
  // service.JobID = JSON.stringify([service.JobID, service.Namespace]);
}
