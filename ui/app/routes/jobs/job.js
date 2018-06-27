import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import notifyError from 'nomad-ui/utils/notify-error';
import PromiseObject from 'nomad-ui/utils/classes/promise-object';
import { qpBuilder } from 'nomad-ui/utils/classes/query-params';

const jobCrumb = job => ({
  label: job.get('trimmedName'),
  args: [
    'jobs.job.index',
    job.get('plainId'),
    qpBuilder({
      jobNamespace: job.get('namespace.name') || 'default',
    }),
  ],
});

export default Route.extend({
  store: service(),
  token: service(),

  breadcrumbs(model) {
    if (!model) return [];

    if (model.get('parent.content')) {
      return [
        PromiseObject.create({
          promise: model.get('parent').then(parent => jobCrumb(parent)),
        }),
        jobCrumb(model),
      ];
    } else {
      return [jobCrumb(model)];
    }
  },

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
});
