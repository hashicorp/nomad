import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import notifyError from 'nomad-ui/utils/notify-error';

// copied from jobs/job, issue to improve: https://github.com/hashicorp/nomad/issues/7458

export default Route.extend({
  store: service(),
  token: service(),

  serialize(model) {
    return { job_name: model.get('plainId') };
  },

  model(params, transition) {
    const namespace = transition.to.queryParams.namespace || this.get('system.activeNamespace.id');
    const name = params.job_name;
    const fullId = JSON.stringify([name, namespace || 'default']);
    return this.store
      .findRecord('job', fullId, { reload: true })
      .then(job => {
        return job.get('allocations').then(() => job);
      })
      .catch(notifyError(this));
  },
});
