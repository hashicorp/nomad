import Ember from 'ember';

const { Route } = Ember;

export default Route.extend({
  model({ name }) {
    // If the job is a partial (from the list request) it won't have task
    // groups. Reload the job to ensure task groups are present.
    return this.modelFor('jobs.job')
      .reload()
      .then(job => {
        return job
          .hasMany('allocations')
          .reload()
          .then(() => {
            return job.get('taskGroups').findBy('name', name);
          });
      });
  },
});
