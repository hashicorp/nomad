import Ember from 'ember';

const { Route } = Ember;

export default Route.extend({
  model() {
    const job = this.modelFor('jobs.job');
    return job.fetchRawDefinition().then(definition => ({
      job,
      definition,
    }));
  },
});
