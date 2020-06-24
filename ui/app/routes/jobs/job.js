import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import notifyError from 'nomad-ui/utils/notify-error';
import { jobCrumbs } from 'nomad-ui/utils/breadcrumb-utils';
import classic from 'ember-classic-decorator';

@classic
export default class JobRoute extends Route {
  @service store;
  @service token;

  breadcrumbs = jobCrumbs;

  serialize(model) {
    return { job_name: model.get('plainId') };
  }

  model(params, transition) {
    const namespace = transition.to.queryParams.namespace || this.get('system.activeNamespace.id');
    const name = params.job_name;
    const fullId = JSON.stringify([name, namespace || 'default']);
    return this.store
      .findRecord('job', fullId, { reload: true })
      .then(job => {
        return RSVP.all([job.get('allocations'), job.get('evaluations')]).then(() => job);
      })
      .catch(notifyError(this));
  }
}
