import Ember from 'ember';
import ApplicationAdapter from './application';

const { RSVP } = Ember;

export default ApplicationAdapter.extend({
  shouldReloadAll: () => true,

  findRecord(store, { modelName }, id, snapshot) {
    // To make a findRecord response reflect the findMany response, the JobSummary
    // from /summary needs to be stitched into the response.
    return RSVP.hash({
      job: this._super(...arguments),
      summary: this.ajax(`${this.buildURL(modelName, id, snapshot, 'findRecord')}/summary`),
    }).then(({ job, summary }) => {
      job.JobSummary = summary;
      return job;
    });
  },

  findAllocations(job) {
    const url = `${this.buildURL('job', job.get('id'), job, 'findRecord')}/allocations`;
    return this.ajax(url, 'GET').then(allocs => {
      return this.store.pushPayload('allocation', {
        allocations: allocs,
      });
    });
  },

  fetchRawDefinition(job) {
    const url = this.buildURL('job', job.get('id'), job, 'findRecord');
    return this.ajax(url, 'GET');
  },
});
