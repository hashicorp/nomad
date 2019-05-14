import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import notifyError from 'nomad-ui/utils/notify-error';
import { jobCrumbs } from 'nomad-ui/utils/breadcrumb-utils';

export default Route.extend({
  store: service(),
  token: service(),

  breadcrumbs: jobCrumbs,

  serialize(model) {
    return { job_name: model.get('plainId') };
  },

  model(params, transition) {
    const namespace = transition.queryParams.namespace || this.get('system.activeNamespace.id');
    const name = params.job_name;
    const fullId = JSON.stringify([name, namespace || 'default']);
    return this.store
      .findRecord('job', fullId, { reload: true })
      .then(job => {
        return RSVP.all([job.get('allocations'), job.get('evaluations')]).then(() => job);
      })
      .catch(notifyError(this));
  },
});
