import { inject as service } from '@ember/service';
import { assign } from '@ember/polyfills';
import Watchable from './watchable';

export default Watchable.extend({
  system: service(),

  buildQuery() {
    const namespace = this.get('system.activeNamespace.id');

    if (namespace && namespace !== 'default') {
      return { namespace };
    }
    return {};
  },

  findAll() {
    const namespace = this.get('system.activeNamespace');
    return this._super(...arguments).then(data => {
      data.forEach(job => {
        job.Namespace = namespace ? namespace.get('id') : 'default';
      });
      return data;
    });
  },

  findRecordSummary(modelName, name, snapshot, namespaceQuery) {
    return this.ajax(`${this.buildURL(modelName, name, snapshot, 'findRecord')}/summary`, 'GET', {
      data: assign(this.buildQuery() || {}, namespaceQuery),
    });
  },

  findRecord(store, type, id, snapshot) {
    const [, namespace] = JSON.parse(id);
    const namespaceQuery = namespace && namespace !== 'default' ? { namespace } : {};

    return this._super(store, type, id, snapshot, namespaceQuery);
  },

  urlForFindRecord(id, type, hash) {
    const [name, namespace] = JSON.parse(id);
    let url = this._super(name, type, hash);
    if (namespace && namespace !== 'default') {
      url += `?namespace=${namespace}`;
    }
    return url;
  },

  xhrKey(url, method, options = {}) {
    const namespace = options.data && options.data.namespace;
    if (namespace) {
      return `${url}?namespace=${namespace}`;
    }
    return url;
  },

  relationshipFallbackLinks: {
    summary: '/summary',
  },

  findAllocations(job) {
    const url = `${this.buildURL('job', job.get('id'), job, 'findRecord')}/allocations`;
    return this.ajax(url, 'GET', { data: this.buildQuery() }).then(allocs => {
      return this.store.pushPayload('allocation', {
        allocations: allocs,
      });
    });
  },

  fetchRawDefinition(job) {
    const url = this.buildURL('job', job.get('id'), job, 'findRecord');
    return this.ajax(url, 'GET', { data: this.buildQuery() });
  },

  forcePeriodic(job) {
    if (job.get('periodic')) {
      const url = addToPath(this.urlForFindRecord(job.get('id'), 'job'), '/periodic/force');
      return this.ajax(url, 'POST');
    }
  },

  stop(job) {
    const url = this.urlForFindRecord(job.get('id'), 'job');
    return this.ajax(url, 'DELETE');
  },
});

function addToPath(url, extension = '') {
  const [path, params] = url.split('?');
  let newUrl = `${path}${extension}`;

  if (params) {
    newUrl += `?${params}`;
  }

  return newUrl;
}
