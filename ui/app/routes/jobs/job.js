import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import notifyError from 'nomad-ui/utils/notify-error';
import { watchRecord, watchRelationship } from 'nomad-ui/utils/properties/watch';

export default Route.extend({
  store: service(),
  token: service(),
  watchList: service(),

  serialize(model) {
    return { job_name: model.get('plainId') };
  },

  model(params, transition) {
    const namespace = transition.queryParams.namespace || this.get('system.activeNamespace.id');
    const name = params.job_name;
    const fullId = JSON.stringify([name, namespace || 'default']);
    return this.get('store')
      .findRecord('job', fullId, { reload: true })
      .then(job => {
        return RSVP.all([job.get('allocations'), job.get('evaluations')]).then(() => job);
      })
      .catch(notifyError(this));
  },

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
});
