import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchRelationship } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default Route.extend(WithWatchers, {
  model() {
    const job = this.modelFor('jobs.job');
    return job && job.get('evaluations').then(() => job);
  },

  startWatchers(controller, model) {
    if (model) {
      controller.set('watchEvaluations', this.get('watchEvaluations').perform(model));
    }
  },

  watchEvaluations: watchRelationship('evaluations'),

  watchers: collect('watchEvaluations'),
});
