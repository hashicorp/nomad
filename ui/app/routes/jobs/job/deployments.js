import Ember from 'ember';

const { Route, RSVP } = Ember;

export default Route.extend({
  model() {
    const job = this.modelFor('jobs.job');
    return RSVP.all([job.get('deployments'), job.get('versions')]).then(() => job);
  },
});
