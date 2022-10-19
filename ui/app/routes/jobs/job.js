import { inject as service } from '@ember/service';
import Route from '@ember/routing/route';
import RSVP from 'rsvp';
import notifyError from 'nomad-ui/utils/notify-error';
import classic from 'ember-classic-decorator';

@classic
export default class JobRoute extends Route {
  @service can;
  @service store;
  @service token;
  @service router;

  serialize(model) {
    return { job_name: model.get('idWithNamespace') };
  }

  model(params) {
    let name,
      namespace = 'default';
    const { job_name } = params;
    const delimiter = job_name.lastIndexOf('@');
    if (delimiter !== -1) {
      name = job_name.slice(0, delimiter);
      namespace = job_name.slice(delimiter + 1);
    } else {
      name = job_name;
    }

    const fullId = JSON.stringify([name, namespace]);

    return this.store
      .findRecord('job', fullId, { reload: true })
      .then((job) => {
        const relatedModelsQueries = [
          job.get('allocations'),
          job.get('evaluations'),
          this.store.query('job', { namespace, meta: true }),
          this.store.findAll('namespace'),
        ];

        if (this.can.can('accept recommendation')) {
          relatedModelsQueries.push(job.get('recommendationSummaries'));
        }

        // Optimizing future node look ups by preemptively loading everything
        if (job.get('hasClientStatus') && this.can.can('read client')) {
          relatedModelsQueries.push(this.store.findAll('node'));
        }

        return RSVP.all(relatedModelsQueries).then(() => job);
      })
      .catch(notifyError(this));
  }
}
