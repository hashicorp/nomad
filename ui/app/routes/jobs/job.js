import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import notifyError from 'nomad-ui/utils/notify-error';
import { jobCrumbs } from 'nomad-ui/utils/breadcrumb-utils';
import classic from 'ember-classic-decorator';

@classic
export default class JobRoute extends Route {
  @service can;
  @service store;
  @service token;

  breadcrumbs = jobCrumbs;

  /*
    We rely on the router bug reported in https://github.com/emberjs/ember.js/issues/18683
    Nomad passes job namespaces to sibling routes using LinkTo and transitions
    These only trigger partial transitions which do not map the query parameters of the previous
    state, the workaround to trigger a full transition is calling refreshModel.
  */
  queryParams = {
    jobNamespace: {
      as: 'namespace',
      refreshModel: true,
    },
  };

  serialize(model) {
    return { job_name: model.get('plainId') };
  }

  model(params, transition) {
    const namespace = transition.to.queryParams.namespace || 'default';
    const name = params.job_name;
    const fullId = JSON.stringify([name, namespace]);

    return this.store
      .findRecord('job', fullId, { reload: true })
      .then(job => {
        const relatedModelsQueries = [
          job.get('allocations'),
          job.get('evaluations'),
          this.store.query('job', { namespace }),
          this.store.findAll('namespace'),
        ];

        if (this.can.can('accept recommendation')) {
          relatedModelsQueries.push(job.get('recommendationSummaries'));
        }

        return RSVP.all(relatedModelsQueries).then(() => job);
      })
      .catch(notifyError(this));
  }
}
