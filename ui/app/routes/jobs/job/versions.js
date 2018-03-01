import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchRelationship } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default Route.extend(WithWatchers, {
  model() {
    const job = this.modelFor('jobs.job');
    return job.get('versions').then(() => job);
  },

  setupController(controller, model) {
    controller.set('watcher', this.get('watchVersions').perform(model));
    return this._super(...arguments);
  },

  watchVersions: watchRelationship('versions'),
  watchers: collect('watchVersions'),
});
