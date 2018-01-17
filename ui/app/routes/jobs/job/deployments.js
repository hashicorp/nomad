import Route from '@ember/routing/route';
import RSVP from 'rsvp';

export default Route.extend({
  model() {
    const job = this.modelFor('jobs.job');
    return RSVP.all([job.get('deployments'), job.get('versions')]).then(() => job);
  },
});
