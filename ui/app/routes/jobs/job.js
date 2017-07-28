import Ember from 'ember';

const { RSVP, Route, inject } = Ember;

export default Route.extend({
  store: inject.service(),

  model({ job_id }) {
    return this.get('store').find('job', job_id).then(job => {
      // Force reload the job to ensure task groups are present
      return RSVP.hash({
        job: job.reload(),
        allocations: job.findAllocations(),
      }).then(({ job }) => job);
    });
  },
});
