import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchRecord, watchRelationship } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default Route.extend(WithWatchers, {
  setupController(controller, model) {
    controller.set('watchers', {
      model: this.get('watch').perform(model),
      summary: this.get('watchSummary').perform(model),
      evaluations: this.get('watchEvaluations').perform(model),
      deployments: this.get('watchDeployments').perform(model),
    });

    return this._super(...arguments);
  },

  watch: watchRecord('job'),
  watchSummary: watchRelationship('summary'),
  watchEvaluations: watchRelationship('evaluations'),
  watchDeployments: watchRelationship('deployments'),

  watchers: collect('watch', 'watchSummary', 'watchEvaluations', 'watchDeployments'),
});
