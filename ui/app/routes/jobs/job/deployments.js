import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import { collect } from '@ember/object/computed';
import { watchRelationship } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default Route.extend(WithWatchers, {
  model() {
    const job = this.modelFor('jobs.job');
    return job && RSVP.all([job.get('deployments'), job.get('versions')]).then(() => job);
  },

  startWatchers(controller, model) {
    if (model) {
      controller.set('watchDeployments', this.watchDeployments.perform(model));
      controller.set('watchVersions', this.watchVersions.perform(model));
    }
  },

  watchDeployments: watchRelationship('deployments'),
  watchVersions: watchRelationship('versions'),

  watchers: collect('watchDeployments', 'watchVersions'),
});
