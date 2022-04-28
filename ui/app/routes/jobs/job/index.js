import Route from '@ember/routing/route';
import { collect } from '@ember/object/computed';
import { watchRecord, watchRelationship, watchQuery } from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default class IndexRoute extends Route.extend(WithWatchers) {
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
      list:
        model.get('hasChildren') &&
        this.watchAllJobs.perform({ namespace: model.namespace.get('name') }),
    });
  }

  setupController(controller, model) {
    // Parameterized and periodic detail pages, which list children jobs,
    // should sort by submit time.
    if (model && ['periodic', 'parameterized'].includes(model.templateType)) {
      controller.setProperties({
        sortProperty: 'submitTime',
        sortDescending: true,
      });
    }
    return super.setupController(...arguments);
  }

  @watchRecord('job') watch;
  @watchQuery('job') watchAllJobs;
  @watchRecord('job-summary') watchSummary;
  @watchRelationship('allocations') watchAllocations;
  @watchRelationship('evaluations') watchEvaluations;
  @watchRelationship('latestDeployment') watchLatestDeployment;

  @collect(
    'watch',
    'watchAllJobs',
    'watchSummary',
    'watchAllocations',
    'watchEvaluations',
    'watchLatestDeployment'
  )
  watchers;
}
