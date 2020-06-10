import Watchable from './watchable';

export default class JobSummary extends Watchable {
  urlForFindRecord(id, type, hash) {
    const [name, namespace] = JSON.parse(id);
    let url = super.urlForFindRecord(name, 'job', hash) + '/summary';
    if (namespace && namespace !== 'default') {
      url += `?namespace=${namespace}`;
    }
    return url;
  }
}
