import Watchable from './watchable';

export default Watchable.extend({
  urlForFindRecord(id, type, hash) {
    const [name, namespace] = JSON.parse(id);
    let url = this._super(name, 'job', hash) + '/summary';
    if (namespace && namespace !== 'default') {
      url += `?namespace=${namespace}`;
    }
    return url;
  },
});
