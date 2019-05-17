import Watchable from './watchable';
import addToPath from 'nomad-ui/utils/add-to-path';

export default Watchable.extend({
  stop: adapterAction('/stop'),
  restart: adapterAction((adapter, allocation) => {
    const prefix = `${adapter.host || '/'}${adapter.urlPrefix()}`;
    return `${prefix}/client/allocation/${allocation.id}/restart`;
  }, 'PUT'),
});

function adapterAction(path, verb = 'POST') {
  return function(allocation) {
    let url;
    if (typeof path === 'function') {
      url = path(this, allocation);
    } else {
      url = addToPath(this.urlForFindRecord(allocation.id, 'allocation'), path);
    }
    return this.ajax(url, verb);
  };
}
