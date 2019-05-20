import Watchable from './watchable';
import addToPath from 'nomad-ui/utils/add-to-path';

export default Watchable.extend({
  stop: adapterAction('/stop'),

  restart(allocation, taskName) {
    const prefix = `${this.host || '/'}${this.urlPrefix()}`;
    const url = `${prefix}/client/allocation/${allocation.id}/restart`;
    return this.ajax(url, 'PUT', {
      data: taskName && { TaskName: taskName },
    });
  },
});

function adapterAction(path, verb = 'POST') {
  return function(allocation) {
    const url = addToPath(this.urlForFindRecord(allocation.id, 'allocation'), path);
    return this.ajax(url, verb);
  };
}
