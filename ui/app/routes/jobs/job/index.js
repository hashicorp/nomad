import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchRecord, watchRelationship, watchAll } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default Route.extend(WithWatchers, {
  startWatchers(controller, model) {
    if (!model) {
      return;
    }
    controller.set('watchers', {
      model: this.watch.perform(model),
      summary: this.watchSummary.perform(model.get('summary')),
      allocations: this.watchAllocations.perform(model),
      evaluations: this.watchEvaluations.perform(model),
      latestDeployment:
        model.get('supportsDeployments') && this.watchLatestDeployment.perform(model),
      list: model.get('hasChildren') && this.watchAll.perform(),
    });
  },

  watch: watchRecord('job'),
  watchAll: watchAll('job'),
  watchSummary: watchRecord('job-summary'),
  watchAllocations: watchRelationship('allocations'),
  watchEvaluations: watchRelationship('evaluations'),
  watchLatestDeployment: watchRelationship('latestDeployment'),

  watchers: collect(
    'watch',
    'watchAll',
    'watchSummary',
    'watchAllocations',
    'watchEvaluations',
    'watchLatestDeployment'
  ),
});
