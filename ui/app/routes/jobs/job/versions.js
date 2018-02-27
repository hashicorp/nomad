import Route from '@ember/routing/route';
import { watchRelationship } from 'nomad-ui/utils/properties/watch';

export default Route.extend({
  model() {
    const job = this.modelFor('jobs.job');
    return job.get('versions').then(() => job);
  },

  setupController(controller, model) {
    controller.set('watcher', this.get('watchVersions').perform(model));
    return this._super(...arguments);
  },

  deactivate() {
    this.get('watchVersions').cancelAll();
    return this._super(...arguments);
  },

  watchVersions: watchRelationship('versions'),
});
