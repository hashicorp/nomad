import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { collect } from '@ember/object/computed';
import {
  watchRecord,
  watchRelationship,
  watchAll,
  watchQuery
} from 'nomad-ui/utils/properties/watch';
import WithWatchers from 'nomad-ui/mixins/with-watchers';

export default class IndexRoute extends Route.extend(WithWatchers) {
  @service can;
  @service store;

  async model() {
    const job = this.modelFor('jobs.job');
    if (!job) {
      return { job, nodes: [] };
    }

    // Optimizing future node look ups by preemptively loading all nodes if
    // necessary and allowed.
    if (this.can.can('read client') && job.get('hasClientStatus')) {
      await this.store.findAll('node');
    }
    const nodes = this.store.peekAll('node');
    return { job, nodes };
  }

  startWatchers(controller, model) {
    if (!model.job) {
      return;
    }
    controller.set('watchers', {
      model: this.watch.perform(model.job),
      summary: this.watchSummary.perform(model.job.get('summary')),
      allocations: this.watchAllocations.perform(model.job),
      evaluations: this.watchEvaluations.perform(model.job),
      latestDeployment:
        model.job.get('supportsDeployments') &&
        this.watchLatestDeployment.perform(model.job),
      list:
        model.job.get('hasChildren') &&
        this.watchAllJobs.perform({
          namespace: model.job.namespace.get('name')
        }),
      nodes:
        this.can.can('read client') &&
        model.job.get('hasClientStatus') &&
        this.watchNodes.perform()
    });
  }

  setupController(controller, model) {
    // Parameterized and periodic detail pages, which list children jobs,
    // should sort by submit time.
    if (
      model.job &&
      ['periodic', 'parameterized'].includes(model.job.templateType)
    ) {
      controller.setProperties({
        sortProperty: 'submitTime',
        sortDescending: true
      });
    }
    return super.setupController(...arguments);
  }

  @watchRecord('job') watch;
  @watchQuery('job') watchAllJobs;
  @watchAll('node') watchNodes;
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
    'watchLatestDeployment',
    'watchNodes'
  )
  watchers;
}
