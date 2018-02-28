import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import { watchRelationship } from 'nomad-ui/utils/properties/watch';

export default Route.extend({
  model() {
    const job = this.modelFor('jobs.job');
    return RSVP.all([job.get('deployments'), job.get('versions')]).then(() => job);
  },

  setupController(controller, model) {
    controller.set('watchDeployments', this.get('watchDeployments').perform(model));
    controller.set('watchVersions', this.get('watchVersions').perform(model));
    return this._super(...arguments);
  },

  deactivate() {
    this.get('watchDeployments').cancelAll();
    this.get('watchVersions').cancelAll();
    return this._super(...arguments);
  },

  watchDeployments: watchRelationship('deployments'),
  watchVersions: watchRelationship('versions'),
});
