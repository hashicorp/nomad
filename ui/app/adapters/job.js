import Ember from 'ember';
import ApplicationAdapter from './application';

const { RSVP, inject, assign } = Ember;

export default ApplicationAdapter.extend({
  system: inject.service(),

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

  findRecord(store, { modelName }, id, snapshot) {
    // To make a findRecord response reflect the findMany response, the JobSummary
    // from /summary needs to be stitched into the response.

    // URL is the form of /job/:name?namespace=:namespace with arbitrary additional query params
    const [name, namespace] = JSON.parse(id);
    const namespaceQuery = namespace && namespace !== 'default' ? { namespace } : {};
    return RSVP.hash({
      job: this.ajax(this.buildURL(modelName, name, snapshot, 'findRecord'), 'GET', {
        data: assign(this.buildQuery() || {}, namespaceQuery),
      }),
      summary: this.ajax(
        `${this.buildURL(modelName, name, snapshot, 'findRecord')}/summary`,
        'GET',
        {
          data: assign(this.buildQuery() || {}, namespaceQuery),
        }
      ),
    }).then(({ job, summary }) => {
      job.JobSummary = summary;
      return job;
    });
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
});
