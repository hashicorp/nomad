import { inject as service } from '@ember/service';
import { assign } from '@ember/polyfills';
import ApplicationAdapter from './application';

export default ApplicationAdapter.extend({
  system: service(),

  shouldReloadAll: () => true,

  buildQuery() {
    const namespace = this.get('system.activeNamespace.id');

    if (namespace && namespace !== 'default') {
      return { namespace };
    }
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
    const [name, namespace] = JSON.parse(id);
    const namespaceQuery = namespace && namespace !== 'default' ? { namespace } : {};

    return this._super(store, type, name, snapshot, namespaceQuery);
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
    const [name, namespace] = JSON.parse(job.get('id'));
    const namespaceQuery = namespace && namespace !== 'default' ? { namespace } : {};
    const url = this.buildURL('job', name, job, 'findRecord');
    return this.ajax(url, 'GET', { data: assign(this.buildQuery() || {}, namespaceQuery) });
  },

  forcePeriodic(job) {
    if (job.get('periodic')) {
      const [name, namespace] = JSON.parse(job.get('id'));
      let url = `${this.buildURL('job', name, job, 'findRecord')}/periodic/force`;

      if (namespace) {
        url += `?namespace=${namespace}`;
      }

      return this.ajax(url, 'POST');
    }
  },
});
