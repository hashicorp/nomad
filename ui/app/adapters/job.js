import Ember from 'ember';
import ApplicationAdapter from './application';

const { RSVP, inject } = Ember;

export default ApplicationAdapter.extend({
  system: inject.service(),

  shouldReloadAll: () => true,

  buildQuery() {
    const namespace = this.get('system.activeNamespace');

    if (namespace) {
      return {
        namespace: namespace.get('name'),
      };
    }
  },

  findAll() {
    const namespace = this.get('system.activeNamespace');
    return this._super(...arguments).then(data => {
      data.forEach(job => {
        job.Namespace = namespace && namespace.get('id');
      });
      return data;
    });
  },

  findRecord(store, { modelName }, id, snapshot) {
    // To make a findRecord response reflect the findMany response, the JobSummary
    // from /summary needs to be stitched into the response.
    return RSVP.hash({
      job: this._super(...arguments),
      summary: this.ajax(`${this.buildURL(modelName, id, snapshot, 'findRecord')}/summary`, 'GET', {
        data: this.buildQuery(),
      }),
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
    const url = this.buildURL('job', job.get('id'), job, 'findRecord');
    return this.ajax(url, 'GET', { data: this.buildQuery() });
  },
});
